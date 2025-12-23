// internal/service/email/network_interface.go
package email

import (
	"context"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// NetworkService interface for email service
type NetworkService interface {
	SendEmail(ctx context.Context, email *models.Email, attachments []AttachmentInfo) error
}
