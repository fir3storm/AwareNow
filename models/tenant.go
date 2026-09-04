package models

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

const (
	// TenantsDBPath is the path to the global tenants metadata database
	TenantsDBPath = "tenants.db"
	// TenantDBDir is the directory where tenant databases are stored
	TenantDBDir = "tenant_dbs"
	// TemplateDBPath is the path to the template database used for new tenants
	TemplateDBPath = "tenant_template.db"
)

var (
	// ErrTenantNotFound is returned when a tenant is not found
	ErrTenantNotFound = errors.New("tenant not found")
	// ErrTenantExists is returned when a tenant with the same domain already exists
	ErrTenantExists = errors.New("tenant with this domain already exists")
	// ErrTenantInactive is returned when an operation is attempted on an inactive tenant
	ErrTenantInactive = errors.New("tenant is inactive")

	tenantManager *TenantManager
	once          sync.Once
)

// Tenant represents an organization/tenant in the multi-tenancy system.
// Each tenant has its own isolated SQLite database.
type Tenant struct {
	ID        uint      `json:"id" gorm:"primary_key"`
	Name      string    `json:"name" gorm:"not null"`
	Domain    string    `json:"domain" gorm:"not null;unique"`
	DBPath    string    `json:"db_path" gorm:"not null"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for the Tenant model
func (Tenant) TableName() string {
	return "tenants"
}

// TenantUser represents a user who can access the super-admin panel
// to manage tenants.
type TenantUser struct {
	ID           uint      `json:"id" gorm:"primary_key"`
	TenantID     uint      `json:"tenant_id" gorm:"default:null"`
	Email        string    `json:"email" gorm:"not null;unique"`
	IsSuperAdmin bool      `json:"is_super_admin" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName specifies the table name for the TenantUser model
func (TenantUser) TableName() string {
	return "tenant_users"
}

// TenantManager manages database connections for all tenants.
// It maintains a pool of database connections and provides methods
// to create, retrieve, and delete tenant-specific databases.
type TenantManager struct {
	mu       sync.RWMutex
	db       *gorm.DB
	conns    map[uint]*gorm.DB
	dbDir    string
}

// GetTenantManager returns the singleton TenantManager instance.
// It initializes the manager on first call.
func GetTenantManager() *TenantManager {
	once.Do(func() {
		tenantManager = &TenantManager{
			conns: make(map[uint]*gorm.DB),
			dbDir: TenantDBDir,
		}
	})
	return tenantManager
}

// Initialize sets up the tenants metadata database and ensures the
// tenant database directory exists.
func (tm *TenantManager) Initialize() error {
	// Ensure tenant database directory exists
	if err := os.MkdirAll(tm.dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create tenant db directory: %v", err)
	}

	// Open the tenants metadata database
	var err error
	tm.db, err = gorm.Open("sqlite3", TenantsDBPath)
	if err != nil {
		return fmt.Errorf("failed to open tenants database: %v", err)
	}
	tm.db.LogMode(false)
	tm.db.SetLogger(log.Logger)
	tm.db.DB().SetMaxOpenConns(1)

	// Auto-migrate the tenants and tenant_users tables
	tm.db.AutoMigrate(&Tenant{}, &TenantUser{})

	log.Info("Tenant manager initialized successfully")
	return nil
}

// Close closes all tenant database connections and the metadata database.
func (tm *TenantManager) Close() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for id, conn := range tm.conns {
		if err := conn.Close(); err != nil {
			log.Errorf("Error closing tenant db connection for tenant %d: %v", id, err)
		}
	}
	tm.conns = make(map[uint]*gorm.DB)

	if tm.db != nil {
		return tm.db.Close()
	}
	return nil
}

// GetConnection returns a GORM database connection for the specified tenant.
// It caches connections to avoid reopening them on every request.
func (tm *TenantManager) GetConnection(tenantID uint) (*gorm.DB, error) {
	tm.mu.RLock()
	conn, exists := tm.conns[tenantID]
	tm.mu.RUnlock()

	if exists {
		return conn, nil
	}

	// Look up the tenant to get the DB path
	tenant, err := tm.GetTenant(tenantID)
	if err != nil {
		return nil, err
	}

	if !tenant.IsActive {
		return nil, ErrTenantInactive
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := tm.conns[tenantID]; exists {
		return conn, nil
	}

	// Open new connection
	conn, err = gorm.Open("sqlite3", tenant.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tenant database: %v", err)
	}
	conn.LogMode(false)
	conn.SetLogger(log.Logger)
	conn.DB().SetMaxOpenConns(1)

	tm.conns[tenantID] = conn
	return conn, nil
}

// CreateTenant creates a new tenant with an isolated SQLite database.
// It copies the schema from a template database if available.
func (tm *TenantManager) CreateTenant(name, domain string) (*Tenant, error) {
	// Validate inputs
	if name == "" {
		return nil, errors.New("tenant name cannot be empty")
	}
	if domain == "" {
		return nil, errors.New("tenant domain cannot be empty")
	}

	// Normalize domain to lowercase
	domain = strings.ToLower(strings.TrimSpace(domain))

	// Check if tenant with this domain already exists
	var count int64
	tm.db.Model(&Tenant{}).Where("domain = ?", domain).Count(&count)
	if count > 0 {
		return nil, ErrTenantExists
	}

	// Generate a unique database path for this tenant
	dbFilename := fmt.Sprintf("tenant_%s_%d.db", strings.ReplaceAll(domain, ".", "_"), time.Now().Unix())
	dbPath := filepath.Join(tm.dbDir, dbFilename)

	// Create the tenant record
	tenant := &Tenant{
		Name:     name,
		Domain:   domain,
		DBPath:   dbPath,
		IsActive: true,
	}

	if err := tm.db.Create(tenant).Error; err != nil {
		return nil, fmt.Errorf("failed to create tenant record: %v", err)
	}

	// Create the isolated database for this tenant
	if err := tm.createTenantDatabase(dbPath); err != nil {
		// Rollback: delete the tenant record
		tm.db.Delete(tenant)
		return nil, fmt.Errorf("failed to create tenant database: %v", err)
	}

	log.Infof("Created tenant '%s' (ID: %d) with domain '%s'", name, tenant.ID, domain)
	return tenant, nil
}

// createTenantDatabase creates a new SQLite database for a tenant.
// It copies the schema from a template database if available, otherwise
// creates a minimal schema.
func (tm *TenantManager) createTenantDatabase(dbPath string) error {
	// Check if template database exists
	if _, err := os.Stat(TemplateDBPath); err == nil {
		// Copy the template database
		return tm.copyTemplateDatabase(dbPath)
	}

	// Create a new database with the standard schema
	return tm.createSchemaFromMigrations(dbPath)
}

// copyTemplateDatabase copies the template database to the specified path.
func (tm *TenantManager) copyTemplateDatabase(dbPath string) error {
	sourceData, err := ioutil.ReadFile(TemplateDBPath)
	if err != nil {
		return fmt.Errorf("failed to read template database: %v", err)
	}

	if err := ioutil.WriteFile(dbPath, sourceData, 0644); err != nil {
		return fmt.Errorf("failed to write tenant database: %v", err)
	}

	log.Infof("Copied template database to %s", dbPath)
	return nil
}

// createSchemaFromMigrations creates a new database by running all migrations.
func (tm *TenantManager) createSchemaFromMigrations(dbPath string) error {
	tenantDB, err := gorm.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open new tenant database: %v", err)
	}
	defer tenantDB.Close()

	// Run migrations from the sqlite3 migrations directory
	migrationsDir := "db/db_sqlite3/migrations"
	files, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %v", err)
	}

	// Sort files to ensure migrations run in order
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}

		migrationPath := filepath.Join(migrationsDir, f.Name())
		content, err := ioutil.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %v", f.Name(), err)
		}

		// Extract the "Up" portion of the migration
		upSQL := extractUpMigration(string(content))
		if upSQL == "" {
			continue
		}

		// Execute each statement
		statements := strings.Split(upSQL, ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if err := tenantDB.Exec(stmt).Error; err != nil {
				return fmt.Errorf("failed to execute migration %s: %v", f.Name(), err)
			}
		}
	}

	log.Infof("Created tenant database with schema at %s", dbPath)
	return nil
}

// extractUpMigration extracts the SQL between "-- +goose Up" and "-- +goose Down" markers.
func extractUpMigration(content string) string {
	lines := strings.Split(content, "\n")
	var upLines []string
	inUp := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose Up") {
			inUp = true
			continue
		}
		if strings.HasPrefix(trimmed, "-- +goose Down") {
			break
		}
		if inUp {
			upLines = append(upLines, line)
		}
	}

	return strings.Join(upLines, "\n")
}

// DeleteTenant deletes a tenant and optionally removes its database file.
func (tm *TenantManager) DeleteTenant(tenantID uint) error {
	tenant, err := tm.GetTenant(tenantID)
	if err != nil {
		return err
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Close the connection if it's open
	if conn, exists := tm.conns[tenantID]; exists {
		conn.Close()
		delete(tm.conns, tenantID)
	}

	// Delete the tenant record
	if err := tm.db.Where("id = ?", tenantID).Delete(&Tenant{}).Error; err != nil {
		return fmt.Errorf("failed to delete tenant record: %v", err)
	}

	// Remove the database file
	if _, err := os.Stat(tenant.DBPath); err == nil {
		if err := os.Remove(tenant.DBPath); err != nil {
			log.Errorf("Failed to remove tenant database file %s: %v", tenant.DBPath, err)
		}
	}

	log.Infof("Deleted tenant '%s' (ID: %d)", tenant.Name, tenantID)
	return nil
}

// GetTenant retrieves a tenant by ID.
func (tm *TenantManager) GetTenant(tenantID uint) (*Tenant, error) {
	tenant := &Tenant{}
	if err := tm.db.Where("id = ?", tenantID).First(tenant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return tenant, nil
}

// GetTenantByDomain retrieves a tenant by its domain name.
func (tm *TenantManager) GetTenantByDomain(domain string) (*Tenant, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	tenant := &Tenant{}
	if err := tm.db.Where("domain = ?", domain).First(tenant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return tenant, nil
}

// ListTenants returns all tenants in the system.
func (tm *TenantManager) ListTenants() ([]Tenant, error) {
	tenants := []Tenant{}
	if err := tm.db.Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

// UpdateTenant updates a tenant's information.
func (tm *TenantManager) UpdateTenant(tenantID uint, updates map[string]interface{}) (*Tenant, error) {
	tenant, err := tm.GetTenant(tenantID)
	if err != nil {
		return nil, err
	}

	// Only allow updating certain fields
	allowedFields := map[string]bool{
		"name":      true,
		"domain":    true,
		"is_active": true,
	}

	filteredUpdates := make(map[string]interface{})
	for key, value := range updates {
		if allowedFields[key] {
			filteredUpdates[key] = value
		}
	}

	if len(filteredUpdates) == 0 {
		return nil, errors.New("no valid fields to update")
	}

	filteredUpdates["updated_at"] = time.Now().UTC()

	if err := tm.db.Model(tenant).Updates(filteredUpdates).Error; err != nil {
		return nil, fmt.Errorf("failed to update tenant: %v", err)
	}

	// Refresh the tenant data
	return tm.GetTenant(tenantID)
}

// GetTenantStats returns statistics for a specific tenant.
func (tm *TenantManager) GetTenantStats(tenantID uint) (map[string]interface{}, error) {
	tenant, err := tm.GetTenant(tenantID)
	if err != nil {
		return nil, err
	}

	if !tenant.IsActive {
		return nil, ErrTenantInactive
	}

	stats := map[string]interface{}{
		"tenant_id":   tenant.ID,
		"name":        tenant.Name,
		"domain":      tenant.Domain,
		"is_active":   tenant.IsActive,
		"created_at":  tenant.CreatedAt,
		"updated_at":  tenant.UpdatedAt,
		"db_path":     tenant.DBPath,
	}

	// Get database file size
	if fileInfo, err := os.Stat(tenant.DBPath); err == nil {
		stats["db_size_bytes"] = fileInfo.Size()
		stats["db_size_human"] = formatBytes(fileInfo.Size())
	}

	// Try to get counts from the tenant database
	tenantDB, err := tm.GetConnection(tenantID)
	if err == nil {
		var userCount, campaignCount, groupCount int64
		tenantDB.Model(&User{}).Count(&userCount)
		tenantDB.Model(&Campaign{}).Count(&campaignCount)
		tenantDB.Model(&Group{}).Count(&groupCount)
		stats["user_count"] = userCount
		stats["campaign_count"] = campaignCount
		stats["group_count"] = groupCount
	}

	return stats, nil
}

// CreateTenantUser creates a new tenant user (super admin).
func (tm *TenantManager) CreateTenantUser(email string, isSuperAdmin bool) (*TenantUser, error) {
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	// Check if user already exists
	var count int64
	tm.db.Model(&TenantUser{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return nil, errors.New("tenant user with this email already exists")
	}

	user := &TenantUser{
		Email:        email,
		IsSuperAdmin: isSuperAdmin,
	}

	if err := tm.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create tenant user: %v", err)
	}

	return user, nil
}

// GetTenantUser retrieves a tenant user by ID.
func (tm *TenantManager) GetTenantUser(userID uint) (*TenantUser, error) {
	user := &TenantUser{}
	if err := tm.db.Where("id = ?", userID).First(user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return user, nil
}

// GetTenantUserByEmail retrieves a tenant user by email.
func (tm *TenantManager) GetTenantUserByEmail(email string) (*TenantUser, error) {
	user := &TenantUser{}
	if err := tm.db.Where("email = ?", email).First(user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return user, nil
}

// ListTenantUsers returns all tenant users.
func (tm *TenantManager) ListTenantUsers() ([]TenantUser, error) {
	users := []TenantUser{}
	if err := tm.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// DeleteTenantUser deletes a tenant user.
func (tm *TenantManager) DeleteTenantUser(userID uint) error {
	if err := tm.db.Where("id = ?", userID).Delete(&TenantUser{}).Error; err != nil {
		return fmt.Errorf("failed to delete tenant user: %v", err)
	}
	return nil
}

// formatBytes converts bytes to a human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
