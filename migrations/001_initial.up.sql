-- migrations/20251224000000_initial_schema.up.sql
-- Initial database schema for MXIL Server

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    master_username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(100),
    bio TEXT,
    password_hash VARCHAR(255) NOT NULL,
    mfa_enabled BOOLEAN DEFAULT FALSE,
    mfa_secret VARCHAR(255),
    recovery_codes TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    is_verified BOOLEAN DEFAULT FALSE,
    verification_token VARCHAR(255),
    verification_expires_at TIMESTAMP,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    last_login TIMESTAMP,
    security_score INTEGER DEFAULT 50,
    storage_quota_used BIGINT DEFAULT 0,
    storage_quota_total BIGINT DEFAULT 1073741824, -- 1GB default
    trusted_devices JSONB DEFAULT '[]',
    auth_methods JSONB DEFAULT '[]',
    
    -- Privacy settings
    metadata_minimization BOOLEAN DEFAULT TRUE,
    logging_consent BOOLEAN DEFAULT TRUE,
    analytics_opt_out BOOLEAN DEFAULT FALSE,
    auto_delete_old_messages BOOLEAN DEFAULT FALSE,
    retention_days INTEGER DEFAULT 365,
    
    -- Security settings
    session_timeout INTEGER DEFAULT 3600, -- 1 hour
    last_security_scan TIMESTAMP,
    
    -- Appearance settings
    ui_theme VARCHAR(50) DEFAULT 'system',
    accent_color VARCHAR(7) DEFAULT '#6d4aff',
    density VARCHAR(50) DEFAULT 'comfortable',
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Indexes
    INDEX idx_users_username (master_username),
    INDEX idx_users_email (email),
    INDEX idx_users_is_active (is_active),
    INDEX idx_users_created_at (created_at)
);

-- Sessions table
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(512) UNIQUE NOT NULL,
    user_agent TEXT,
    ip_address INET,
    device_fingerprint VARCHAR(255),
    expires_at TIMESTAMP NOT NULL,
    last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_sessions_user_id (user_id),
    INDEX idx_sessions_token (token),
    INDEX idx_sessions_expires_at (expires_at)
);

-- Password reset tokens
CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_password_reset_tokens_token (token),
    INDEX idx_password_reset_tokens_user_id (user_id)
);

-- Folders table
CREATE TABLE folders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES folders(id) ON DELETE SET NULL,
    name VARCHAR(100) NOT NULL,
    path VARCHAR(500) NOT NULL,
    is_system BOOLEAN DEFAULT FALSE,
    email_count INTEGER DEFAULT 0,
    unread_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(user_id, path),
    
    -- Indexes
    INDEX idx_folders_user_id (user_id),
    INDEX idx_folders_parent_id (parent_id),
    INDEX idx_folders_path (path)
);

-- Labels table
CREATE TABLE labels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    color VARCHAR(7),
    icon VARCHAR(50),
    is_system BOOLEAN DEFAULT FALSE,
    email_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(user_id, name),
    
    -- Indexes
    INDEX idx_labels_user_id (user_id)
);

-- Contacts table
CREATE TABLE contacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(200),
    email_address VARCHAR(255) NOT NULL,
    public_keys TEXT[],
    network_identities JSONB DEFAULT '{}',
    notes TEXT,
    is_trusted BOOLEAN DEFAULT FALSE,
    last_contacted TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(user_id, email_address),
    
    -- Indexes
    INDEX idx_contacts_user_id (user_id),
    INDEX idx_contacts_email (email_address),
    INDEX idx_contacts_is_trusted (is_trusted)
);

-- Network identities table
CREATE TABLE network_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    network VARCHAR(50) NOT NULL,
    address VARCHAR(500) NOT NULL,
    is_primary BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    forward_to VARCHAR(500),
    config JSONB DEFAULT '{}',
    last_used TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(user_id, network, address),
    
    -- Indexes
    INDEX idx_network_identities_user_id (user_id),
    INDEX idx_network_identities_network (network),
    INDEX idx_network_identities_is_primary (is_primary)
);

-- Provider bridges table
CREATE TABLE provider_bridges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_type VARCHAR(50) NOT NULL,
    account_identifier VARCHAR(255) NOT NULL,
    encrypted_credentials TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    sync_interval INTEGER DEFAULT 300, -- 5 minutes
    last_sync TIMESTAMP,
    sync_status VARCHAR(50),
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(user_id, provider_type, account_identifier),
    
    -- Indexes
    INDEX idx_provider_bridges_user_id (user_id),
    INDEX idx_provider_bridges_provider_type (provider_type)
);

-- Encryption keys table
CREATE TABLE encryption_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_type VARCHAR(50) NOT NULL,
    key_id VARCHAR(100) UNIQUE NOT NULL,
    public_key TEXT NOT NULL,
    private_key_encrypted TEXT NOT NULL,
    fingerprint VARCHAR(64) NOT NULL,
    is_primary BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    expires_at TIMESTAMP,
    last_used TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Indexes
    INDEX idx_encryption_keys_user_id (user_id),
    INDEX idx_encryption_keys_key_id (key_id),
    INDEX idx_encryption_keys_fingerprint (fingerprint),
    INDEX idx_encryption_keys_is_primary (is_primary)
);

-- Emails table
CREATE TABLE emails (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    thread_id UUID NOT NULL,
    folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
    
    -- Message details
    message_id VARCHAR(500),
    "from" TEXT NOT NULL,
    "to" TEXT[] NOT NULL,
    cc TEXT[] DEFAULT '{}',
    bcc TEXT[] DEFAULT '{}',
    subject TEXT,
    body_plain TEXT,
    body_html TEXT,
    body_markdown TEXT,
    
    -- Metadata
    network VARCHAR(50) NOT NULL,
    priority INTEGER DEFAULT 0,
    is_read BOOLEAN DEFAULT FALSE,
    is_starred BOOLEAN DEFAULT FALSE,
    is_archived BOOLEAN DEFAULT FALSE,
    is_spam BOOLEAN DEFAULT FALSE,
    is_encrypted BOOLEAN DEFAULT FALSE,
    encryption_keys TEXT[] DEFAULT '{}',
    attachments JSONB DEFAULT '[]',
    headers JSONB DEFAULT '{}',
    
    -- References
    in_reply_to VARCHAR(500),
    references TEXT[] DEFAULT '{}',
    
    -- Timestamps
    sent_at TIMESTAMP,
    received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Full-text search
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english', 
            COALESCE(subject, '') || ' ' || 
            COALESCE(body_plain, '') || ' ' ||
            COALESCE("from", '')
        )
    ) STORED,
    
    -- Indexes
    INDEX idx_emails_user_id (user_id),
    INDEX idx_emails_thread_id (thread_id),
    INDEX idx_emails_folder_id (folder_id),
    INDEX idx_emails_network (network),
    INDEX idx_emails_is_read (is_read),
    INDEX idx_emails_is_starred (is_starred),
    INDEX idx_emails_received_at (received_at),
    INDEX idx_emails_search_vector (search_vector) USING GIN
);

-- Email labels junction table
CREATE TABLE email_labels (
    email_id UUID NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (email_id, label_id),
    
    -- Indexes
    INDEX idx_email_labels_email_id (email_id),
    INDEX idx_email_labels_label_id (label_id)
);

-- Attachments table
CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email_id UUID NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename VARCHAR(500) NOT NULL,
    content_type VARCHAR(100),
    size BIGINT NOT NULL,
    storage_path VARCHAR(1000),
    storage_backend VARCHAR(50) DEFAULT 'local',
    checksum VARCHAR(64),
    is_inline BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_attachments_email_id (email_id),
    INDEX idx_attachments_user_id (user_id),
    INDEX idx_attachments_storage_backend (storage_backend)
);

-- Audit logs table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    details JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_audit_logs_user_id (user_id),
    INDEX idx_audit_logs_action (action),
    INDEX idx_audit_logs_created_at (created_at)
);

-- Statistics table
CREATE TABLE statistics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    metric_name VARCHAR(100) NOT NULL,
    metric_value JSONB NOT NULL,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(metric_name, recorded_at),
    
    -- Indexes
    INDEX idx_statistics_metric_name (metric_name),
    INDEX idx_statistics_recorded_at (recorded_at)
);

-- Create system folders for existing users
CREATE OR REPLACE FUNCTION create_system_folders() 
RETURNS TRIGGER AS $$
BEGIN
    -- Create system folders for new user
    INSERT INTO folders (id, user_id, name, path, is_system) VALUES
    (uuid_generate_v4(), NEW.id, 'Inbox', 'Inbox', TRUE),
    (uuid_generate_v4(), NEW.id, 'Sent', 'Sent', TRUE),
    (uuid_generate_v4(), NEW.id, 'Drafts', 'Drafts', TRUE),
    (uuid_generate_v4(), NEW.id, 'Spam', 'Spam', TRUE),
    (uuid_generate_v4(), NEW.id, 'Trash', 'Trash', TRUE),
    (uuid_generate_v4(), NEW.id, 'Archive', 'Archive', TRUE);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to create system folders on user creation
CREATE TRIGGER create_user_folders 
AFTER INSERT ON users 
FOR EACH ROW 
EXECUTE FUNCTION create_system_folders();

-- Update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_folders_updated_at BEFORE UPDATE ON folders FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_labels_updated_at BEFORE UPDATE ON labels FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_contacts_updated_at BEFORE UPDATE ON contacts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_network_identities_updated_at BEFORE UPDATE ON network_identities FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_provider_bridges_updated_at BEFORE UPDATE ON provider_bridges FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_encryption_keys_updated_at BEFORE UPDATE ON encryption_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_emails_updated_at BEFORE UPDATE ON emails FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create indexes for better performance
CREATE INDEX CONCURRENTLY idx_emails_full_text ON emails USING GIN(search_vector);
CREATE INDEX CONCURRENTLY idx_emails_date_user ON emails(user_id, received_at DESC);
CREATE INDEX CONCURRENTLY idx_sessions_expiry ON sessions(expires_at) WHERE expires_at < NOW();