// internal/models/user.go
package models

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}

// TableName specifies the table name
func (NetworkIdentity) TableName() string {
	return "network_identities"
}

// TableName specifies the table name
func (AuthMethod) TableName() string {
	return "auth_methods"
}

// TableName specifies the table name
func (TrustedDevice) TableName() string {
	return "trusted_devices"
}

// TableName specifies the table name
func (ProviderBridge) TableName() string {
	return "provider_bridges"
}

// TableName specifies the table name
func (Session) TableName() string {
	return "sessions"
}
