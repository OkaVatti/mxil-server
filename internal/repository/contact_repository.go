package repository

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// ContactRepository handles contact database operations
type ContactRepository struct {
	db *gorm.DB
}

// NewContactRepository creates a new contact repository
func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

// Create creates a new contact
func (r *ContactRepository) Create(ctx context.Context, contact *models.Contact) error {
	return r.db.WithContext(ctx).Create(contact).Error
}

// GetByID gets a contact by ID
func (r *ContactRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Contact, error) {
	var contact models.Contact
	if err := r.db.WithContext(ctx).
		First(&contact, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}

// GetByEmail gets a contact by email for a user
func (r *ContactRepository) GetByEmail(ctx context.Context, userID uuid.UUID, email string) (*models.Contact, error) {
	var contact models.Contact
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND email_address = ?", userID, email).
		First(&contact).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}

// Update updates a contact
func (r *ContactRepository) Update(ctx context.Context, contact *models.Contact) error {
	return r.db.WithContext(ctx).Save(contact).Error
}

// Delete deletes a contact
func (r *ContactRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.Contact{}, "id = ?", id).Error
}

// List lists contacts for a user with pagination
func (r *ContactRepository) List(ctx context.Context, userID uuid.UUID, limit, offset int, trustedOnly, recentOnly bool) ([]models.Contact, int64, error) {
	var contacts []models.Contact
	query := r.db.WithContext(ctx).Model(&models.Contact{}).
		Where("user_id = ?", userID)

	if trustedOnly {
		query = query.Where("is_trusted = ?", true)
	}

	if recentOnly {
		thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
		query = query.Where("created_at >= ?", thirtyDaysAgo)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply limit and offset
	if err := query.
		Order("name ASC, created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&contacts).Error; err != nil {
		return nil, 0, err
	}

	return contacts, total, nil
}

// Search searches contacts by name or email
func (r *ContactRepository) Search(ctx context.Context, userID uuid.UUID, query string, limit int) ([]models.Contact, error) {
	var contacts []models.Contact
	searchTerm := "%" + strings.ToLower(query) + "%"

	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND (LOWER(name) LIKE ? OR LOWER(email_address) LIKE ?)",
			userID, searchTerm, searchTerm).
		Order("name ASC").
		Limit(limit).
		Find(&contacts).Error; err != nil {
		return nil, err
	}

	return contacts, nil
}

// Import imports contacts from various formats
func (r *ContactRepository) Import(ctx context.Context, userID uuid.UUID, format string, src io.Reader) (*ImportStats, error) {
	stats := &ImportStats{}

	switch strings.ToLower(format) {
	case "vcard":
		return r.importVCard(ctx, userID, src, stats)
	case "csv":
		return r.importCSV(ctx, userID, src, stats)
	case "json":
		return r.importJSON(ctx, userID, src, stats)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// Export exports contacts in specified format
func (r *ContactRepository) Export(ctx context.Context, userID uuid.UUID, format string) ([]byte, error) {
	contacts, err := r.ListAll(ctx, userID)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(format) {
	case "vcard":
		return r.exportVCard(contacts)
	case "csv":
		return r.exportCSV(contacts)
	case "json":
		return r.exportJSON(contacts)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// Helper methods for import/export
func (r *ContactRepository) ListAll(ctx context.Context, userID uuid.UUID) ([]models.Contact, error) {
	var contacts []models.Contact
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("name ASC").
		Find(&contacts).Error; err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *ContactRepository) importVCard(ctx context.Context, userID uuid.UUID, src io.Reader, stats *ImportStats) (*ImportStats, error) {
	// TODO: Implement vCard import
	return stats, fmt.Errorf("vCard import not implemented")
}

func (r *ContactRepository) importCSV(ctx context.Context, userID uuid.UUID, src io.Reader, stats *ImportStats) (*ImportStats, error) {
	reader := csv.NewReader(src)
	reader.TrimLeadingSpace = true

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return stats, err
	}

	// Process rows
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			stats.Failed++
			continue
		}

		stats.Total++
		contact := &models.Contact{
			ID:        uuid.New(),
			UserID:    userID,
			CreatedAt: time.Now(),
		}

		// Map CSV columns to contact fields
		for i, header := range headers {
			if i >= len(row) {
				break
			}

			switch strings.ToLower(header) {
			case "name":
				contact.Name = row[i]
			case "email", "email_address":
				contact.EmailAddress = row[i]
			case "notes":
				contact.Notes = row[i]
			}
		}

		// Check if contact already exists
		existing, err := r.GetByEmail(ctx, userID, contact.EmailAddress)
		if err == nil && existing != nil {
			stats.Duplicates++
			continue
		}

		// Create contact
		if err := r.Create(ctx, contact); err != nil {
			stats.Failed++
		} else {
			stats.Imported++
		}
	}

	return stats, nil
}

func (r *ContactRepository) importJSON(ctx context.Context, userID uuid.UUID, src io.Reader, stats *ImportStats) (*ImportStats, error) {
	var contacts []struct {
		Name         string   `json:"name"`
		EmailAddress string   `json:"email_address"`
		PublicKeys   []string `json:"public_keys"`
		Notes        string   `json:"notes"`
	}

	if err := json.NewDecoder(src).Decode(&contacts); err != nil {
		return stats, err
	}

	for _, data := range contacts {
		stats.Total++

		// Check if contact already exists
		existing, err := r.GetByEmail(ctx, userID, data.EmailAddress)
		if err == nil && existing != nil {
			stats.Duplicates++
			continue
		}

		contact := &models.Contact{
			ID:           uuid.New(),
			UserID:       userID,
			Name:         data.Name,
			EmailAddress: data.EmailAddress,
			PublicKeys:   models.StringArray(data.PublicKeys),
			Notes:        data.Notes,
			CreatedAt:    time.Now(),
		}

		if err := r.Create(ctx, contact); err != nil {
			stats.Failed++
		} else {
			stats.Imported++
		}
	}

	return stats, nil
}

func (r *ContactRepository) exportVCard(contacts []models.Contact) ([]byte, error) {
	var vcards []string
	for _, contact := range contacts {
		vcards = append(vcards, fmt.Sprintf(
			"BEGIN:VCARD\n"+
				"VERSION:3.0\n"+
				"FN:%s\n"+
				"EMAIL:%s\n"+
				"NOTE:%s\n"+
				"END:VCARD",
			contact.Name,
			contact.EmailAddress,
			contact.Notes,
		))
	}
	return []byte(strings.Join(vcards, "\n")), nil
}

func (r *ContactRepository) exportCSV(contacts []models.Contact) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	writer.Write([]string{"name", "email_address", "notes"})

	// Write contacts
	for _, contact := range contacts {
		writer.Write([]string{
			contact.Name,
			contact.EmailAddress,
			contact.Notes,
		})
	}

	writer.Flush()
	return []byte(buf.String()), nil
}

func (r *ContactRepository) exportJSON(contacts []models.Contact) ([]byte, error) {
	type exportContact struct {
		Name         string   `json:"name"`
		EmailAddress string   `json:"email_address"`
		PublicKeys   []string `json:"public_keys"`
		Notes        string   `json:"notes"`
		IsTrusted    bool     `json:"is_trusted"`
		CreatedAt    string   `json:"created_at"`
	}

	var exportContacts []exportContact
	for _, contact := range contacts {
		exportContacts = append(exportContacts, exportContact{
			Name:         contact.Name,
			EmailAddress: contact.EmailAddress,
			PublicKeys:   []string(contact.PublicKeys),
			Notes:        contact.Notes,
			IsTrusted:    contact.IsTrusted,
			CreatedAt:    contact.CreatedAt.Format(time.RFC3339),
		})
	}

	return json.MarshalIndent(exportContacts, "", "  ")
}

// ImportStats contains import statistics
type ImportStats struct {
	Total      int `json:"total"`
	Imported   int `json:"imported"`
	Skipped    int `json:"skipped"`
	Duplicates int `json:"duplicates"`
	Failed     int `json:"failed"`
}
