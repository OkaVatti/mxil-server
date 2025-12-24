-- migrations/001_initial.down.sql
-- Rollback initial database schema

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS encryption_keys;
DROP TABLE IF EXISTS provider_bridges;
DROP TABLE IF EXISTS network_identities;
DROP TABLE IF EXISTS contacts;
DROP TABLE IF EXISTS email_labels;
DROP TABLE IF EXISTS labels;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS emails;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS "uuid-ossp";