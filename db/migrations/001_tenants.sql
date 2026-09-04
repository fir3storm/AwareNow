-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Multi-tenancy support: creates tables for tenant metadata management
-- These tables are stored in the global tenants.db database

-- tenants table stores metadata about each organization/tenant
-- Each tenant has its own isolated SQLite database for data separation
CREATE TABLE IF NOT EXISTS "tenants" (
	"id" integer primary key autoincrement,
	"name" varchar(255) NOT NULL,
	"domain" varchar(255) NOT NULL UNIQUE,
	"db_path" varchar(512) NOT NULL,
	"is_active" boolean NOT NULL DEFAULT 1,
	"created_at" datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
	"updated_at" datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index on domain for fast lookups when resolving tenants by subdomain
CREATE INDEX IF NOT EXISTS "idx_tenants_domain" ON "tenants" ("domain");

-- Index on is_active for filtering active tenants
CREATE INDEX IF NOT EXISTS "idx_tenants_is_active" ON "tenants" ("is_active");

-- tenant_users table stores super admin users who can manage tenants
-- These users have access to the global tenant management interface
CREATE TABLE IF NOT EXISTS "tenant_users" (
	"id" integer primary key autoincrement,
	"tenant_id" integer DEFAULT NULL,
	"email" varchar(255) NOT NULL UNIQUE,
	"is_super_admin" boolean NOT NULL DEFAULT 0,
	"created_at" datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY ("tenant_id") REFERENCES "tenants" ("id") ON DELETE SET NULL
);

-- Index on tenant_id for filtering users by tenant
CREATE INDEX IF NOT EXISTS "idx_tenant_users_tenant_id" ON "tenant_users" ("tenant_id");

-- Index on is_super_admin for filtering super admins
CREATE INDEX IF NOT EXISTS "idx_tenant_users_is_super_admin" ON "tenant_users" ("is_super_admin");

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- WARNING: This will delete all tenant metadata. Tenant databases will need to be cleaned up manually.

DROP INDEX IF EXISTS "idx_tenant_users_is_super_admin";
DROP INDEX IF EXISTS "idx_tenant_users_tenant_id";
DROP TABLE IF EXISTS "tenant_users";

DROP INDEX IF EXISTS "idx_tenants_is_active";
DROP INDEX IF EXISTS "idx_tenants_domain";
DROP TABLE IF EXISTS "tenants";
