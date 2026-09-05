package models

import (
	"errors"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
)

// DeviceFingerprint stores browser fingerprinting data collected from
// campaign recipients when they interact with landing pages.
type DeviceFingerprint struct {
	ID          int64     `json:"id"`
	CampaignID  int64     `json:"campaign_id"`
	RId         string    `json:"r_id"`
	Fingerprint string    `json:"fingerprint" sql:"type:text"`
	UserAgent   string    `json:"user_agent" sql:"type:text"`
	ScreenRes   string    `json:"screen_resolution"`
	ColorDepth  int       `json:"color_depth"`
	Timezone    string    `json:"timezone"`
	Language    string    `json:"language"`
	Platform    string    `json:"platform"`
	Cookies     string    `json:"cookies" sql:"type:text"`
	CreatedAt   time.Time `json:"created_at"`
}

// Session represents a single visit/engagement session by a campaign
// recipient, aggregating behavior events and device information.
type Session struct {
	ID                int64     `json:"id"`
	SessionID         string    `json:"session_id" gorm:"column:session_id"`
	RId               string    `json:"r_id"`
	CampaignID        int64     `json:"campaign_id"`
	StartedAt         time.Time `json:"started_at"`
	EndedAt           time.Time `json:"ended_at"`
	Duration          int       `json:"duration"`
	PagesViewed       int       `json:"pages_viewed"`
	EventsCount       int       `json:"events_count"`
	DeviceFingerprint string    `json:"device_fingerprint"`
}

// ErrDeviceFingerprintNotFound indicates no fingerprint was found for the given criteria
var ErrDeviceFingerprintNotFound = errors.New("device fingerprint not found")

// ErrSessionNotFound indicates no session was found for the given criteria
var ErrSessionNotFound = errors.New("session not found")

// --- DeviceFingerprint CRUD ---

// CreateDeviceFingerprint inserts a new device fingerprint into the database
func CreateDeviceFingerprint(df *DeviceFingerprint) error {
	df.CreatedAt = time.Now().UTC()
	err := db.Save(df).Error
	if err != nil {
		log.Errorf("error creating device fingerprint: %v", err)
		return err
	}
	return nil
}

// GetDeviceFingerprintByID retrieves a device fingerprint by its primary key
func GetDeviceFingerprintByID(id int64) (DeviceFingerprint, error) {
	df := DeviceFingerprint{}
	err := db.Where("id = ?", id).First(&df).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return df, ErrDeviceFingerprintNotFound
		}
		log.Errorf("error getting device fingerprint by id: %v", err)
		return df, err
	}
	return df, nil
}

// GetDeviceFingerprintByRId retrieves the most recent device fingerprint for a recipient
func GetDeviceFingerprintByRId(rid string) (DeviceFingerprint, error) {
	df := DeviceFingerprint{}
	err := db.Where("r_id = ?", rid).Order("created_at desc").First(&df).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return df, ErrDeviceFingerprintNotFound
		}
		log.Errorf("error getting device fingerprint by r_id: %v", err)
		return df, err
	}
	return df, nil
}

// GetDeviceFingerprintsByCampaign retrieves all device fingerprints for a campaign
func GetDeviceFingerprintsByCampaign(campaignID int64) ([]DeviceFingerprint, error) {
	dfs := []DeviceFingerprint{}
	err := db.Where("campaign_id = ?", campaignID).Order("created_at desc").Find(&dfs).Error
	if err != nil {
		log.Errorf("error getting device fingerprints by campaign: %v", err)
		return dfs, err
	}
	return dfs, nil
}

// UpdateDeviceFingerprint updates an existing device fingerprint record
func UpdateDeviceFingerprint(df *DeviceFingerprint) error {
	err := db.Model(df).Where("id = ?", df.ID).Updates(df).Error
	if err != nil {
		log.Errorf("error updating device fingerprint: %v", err)
		return err
	}
	return nil
}

// DeleteDeviceFingerprint removes a device fingerprint by its primary key
func DeleteDeviceFingerprint(id int64) error {
	err := db.Where("id = ?", id).Delete(&DeviceFingerprint{}).Error
	if err != nil {
		log.Errorf("error deleting device fingerprint: %v", err)
		return err
	}
	return nil
}

// DeleteDeviceFingerprintsByCampaign removes all device fingerprints for a campaign
func DeleteDeviceFingerprintsByCampaign(campaignID int64) error {
	err := db.Where("campaign_id = ?", campaignID).Delete(&DeviceFingerprint{}).Error
	if err != nil {
		log.Errorf("error deleting device fingerprints by campaign: %v", err)
		return err
	}
	return nil
}

// DeleteBehaviorEventsByCampaign removes all behavior events for a campaign.
// BehaviorEvent itself is defined in behavior.go (the schema actually wired
// to the live tracking pipeline); this cleanup helper lives here alongside
// the other campaign-teardown helpers in this file.
func DeleteBehaviorEventsByCampaign(campaignID int64) error {
	err := db.Where("campaign_id = ?", campaignID).Delete(&BehaviorEvent{}).Error
	if err != nil {
		log.Errorf("error deleting behavior events by campaign: %v", err)
		return err
	}
	return nil
}

// --- Session CRUD ---

// CreateSession inserts a new session into the database
func CreateSession(s *Session) error {
	err := db.Save(s).Error
	if err != nil {
		log.Errorf("error creating session: %v", err)
		return err
	}
	return nil
}

// GetSessionByID retrieves a session by its primary key
func GetSessionByID(id int64) (Session, error) {
	s := Session{}
	err := db.Where("id = ?", id).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return s, ErrSessionNotFound
		}
		log.Errorf("error getting session by id: %v", err)
		return s, err
	}
	return s, nil
}

// GetSessionBySessionID retrieves a session by its session identifier string
func GetSessionBySessionID(sessionID string) (Session, error) {
	s := Session{}
	err := db.Where("session_id = ?", sessionID).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return s, ErrSessionNotFound
		}
		log.Errorf("error getting session by session id: %v", err)
		return s, err
	}
	return s, nil
}

// GetSessionsByRId retrieves all sessions for a recipient
func GetSessionsByRId(rid string) ([]Session, error) {
	sessions := []Session{}
	err := db.Where("r_id = ?", rid).Order("started_at desc").Find(&sessions).Error
	if err != nil {
		log.Errorf("error getting sessions by r_id: %v", err)
		return sessions, err
	}
	return sessions, nil
}

// GetSessionsByCampaign retrieves all sessions for a campaign
func GetSessionsByCampaign(campaignID int64) ([]Session, error) {
	sessions := []Session{}
	err := db.Where("campaign_id = ?", campaignID).Order("started_at desc").Find(&sessions).Error
	if err != nil {
		log.Errorf("error getting sessions by campaign: %v", err)
		return sessions, err
	}
	return sessions, nil
}

// UpdateSession updates an existing session record, including computing
// the duration from started_at and ended_at if both are set
func UpdateSession(s *Session) error {
	if !s.StartedAt.IsZero() && !s.EndedAt.IsZero() {
		s.Duration = int(s.EndedAt.Sub(s.StartedAt).Seconds())
	}
	err := db.Model(s).Where("id = ?", s.ID).Updates(s).Error
	if err != nil {
		log.Errorf("error updating session: %v", err)
		return err
	}
	return nil
}

// EndSession marks the end of a session, updating the ended_at timestamp,
// computing the duration, and incrementing the events count
func EndSession(s *Session) error {
	s.EndedAt = time.Now().UTC()
	if !s.StartedAt.IsZero() {
		s.Duration = int(s.EndedAt.Sub(s.StartedAt).Seconds())
	}
	err := db.Model(s).Where("id = ?", s.ID).
		Updates(map[string]interface{}{
			"ended_at":     s.EndedAt,
			"duration":     s.Duration,
			"events_count": s.EventsCount,
		}).Error
	if err != nil {
		log.Errorf("error ending session: %v", err)
		return err
	}
	return nil
}

// IncrementSessionEvents increments the events_count for a session
func IncrementSessionEvents(sessionID int64) error {
	err := db.Model(&Session{}).Where("id = ?", sessionID).
		UpdateColumn("events_count", gorm.Expr("events_count + ?", 1)).Error
	if err != nil {
		log.Errorf("error incrementing session events: %v", err)
		return err
	}
	return nil
}

// IncrementSessionPagesViewed increments the pages_viewed count for a session
func IncrementSessionPagesViewed(sessionID int64) error {
	err := db.Model(&Session{}).Where("id = ?", sessionID).
		UpdateColumn("pages_viewed", gorm.Expr("pages_viewed + ?", 1)).Error
	if err != nil {
		log.Errorf("error incrementing session pages viewed: %v", err)
		return err
	}
	return nil
}

// DeleteSession removes a session by its primary key
func DeleteSession(id int64) error {
	err := db.Where("id = ?", id).Delete(&Session{}).Error
	if err != nil {
		log.Errorf("error deleting session: %v", err)
		return err
	}
	return nil
}

// DeleteSessionsByCampaign removes all sessions for a campaign
func DeleteSessionsByCampaign(campaignID int64) error {
	err := db.Where("campaign_id = ?", campaignID).Delete(&Session{}).Error
	if err != nil {
		log.Errorf("error deleting sessions by campaign: %v", err)
		return err
	}
	return nil
}

// GetAverageSessionDuration calculates the average session duration for a campaign
func GetAverageSessionDuration(campaignID int64) (float64, error) {
	var avgDuration struct {
		Avg float64
	}
	err := db.Table("sessions").
		Where("campaign_id = ? AND duration > 0", campaignID).
		Select("avg(duration) as avg").
		Scan(&avgDuration).Error
	if err != nil {
		log.Errorf("error calculating average session duration: %v", err)
		return 0, err
	}
	return avgDuration.Avg, nil
}
