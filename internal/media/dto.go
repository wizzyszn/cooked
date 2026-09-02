package media

import (
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"time"
)

type InitializeRequest struct {
	Purpose     domain.MediaPurpose     `json:"purpose" binding:"required"`
	Filename    string                  `json:"filename" binding:"omitempty,max=255"`
	MIMEType    string                  `json:"mime_type" binding:"required,max=64"`
	ByteSize    int64                   `json:"byte_size" binding:"required,min=1,max=5242880"`
	AccessScope domain.MediaAccessScope `json:"access_scope" binding:"required"`
}
type UploadTicket struct {
	AssetID         uuid.UUID         `json:"asset_id"`
	UploadURL       string            `json:"upload_url"`
	ExpiresAt       time.Time         `json:"expires_at"`
	RequiredHeaders map[string]string `json:"required_headers"`
}
type AssetResponse struct {
	ID               uuid.UUID                    `json:"id"`
	Purpose          domain.MediaPurpose          `json:"purpose"`
	ProcessingStatus domain.MediaProcessingStatus `json:"processing_status"`
	ModerationStatus domain.MediaModerationStatus `json:"moderation_status"`
	AccessScope      domain.MediaAccessScope      `json:"access_scope"`
	MIMEType         *string                      `json:"mime_type,omitempty"`
	ByteSize         *int64                       `json:"byte_size,omitempty"`
	Width            *int                         `json:"width,omitempty"`
	Height           *int                         `json:"height,omitempty"`
	URL              string                       `json:"url,omitempty"`
	Variants         []VariantResponse            `json:"variants,omitempty"`
}
type VariantResponse struct {
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	URL      string `json:"url"`
}
