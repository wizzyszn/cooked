package domain

import (
	"time"

	"github.com/google/uuid"
)

type MediaPurpose string
type MediaProcessingStatus string
type MediaModerationStatus string
type MediaAccessScope string

const (
	MediaPurposeProfileAvatar    MediaPurpose = "profile_avatar"
	MediaPurposeDishCover        MediaPurpose = "dish_cover"
	MediaPurposeRecipeCover      MediaPurpose = "recipe_cover"
	MediaPurposeStepImage        MediaPurpose = "step_image"
	MediaPurposeReviewPhoto      MediaPurpose = "review_photo"
	MediaPurposeCookSessionPhoto MediaPurpose = "cook_session_photo"
)

const (
	MediaAwaitingUpload MediaProcessingStatus = "awaiting_upload"
	MediaUploaded       MediaProcessingStatus = "uploaded"
	MediaProcessing     MediaProcessingStatus = "processing"
	MediaReady          MediaProcessingStatus = "ready"
	MediaRetry          MediaProcessingStatus = "retry"
	MediaFailed         MediaProcessingStatus = "failed"
	MediaDeleted        MediaProcessingStatus = "deleted"
)

const (
	MediaModerationPending  MediaModerationStatus = "pending"
	MediaModerationApproved MediaModerationStatus = "approved"
	MediaModerationRejected MediaModerationStatus = "rejected"
	MediaPublic             MediaAccessScope      = "public"
	MediaPrivate            MediaAccessScope      = "private"
)

type MediaAsset struct {
	BaseModel
	OwnerID          *uuid.UUID            `gorm:"type:uuid;index" json:"owner_id,omitempty"`
	Purpose          MediaPurpose          `gorm:"size:32;not null" json:"purpose"`
	ObjectKey        string                `gorm:"size:512;not null;uniqueIndex" json:"-"`
	OriginalFilename *string               `gorm:"size:255" json:"original_filename,omitempty"`
	DeclaredMIMEType string                `gorm:"size:64;not null" json:"declared_mime_type"`
	DecodedMIMEType  *string               `gorm:"size:64" json:"decoded_mime_type,omitempty"`
	ByteSize         *int64                `json:"byte_size,omitempty"`
	Width            *int                  `json:"width,omitempty"`
	Height           *int                  `json:"height,omitempty"`
	ProcessingStatus MediaProcessingStatus `gorm:"size:24;not null" json:"processing_status"`
	ModerationStatus MediaModerationStatus `gorm:"size:24;not null" json:"moderation_status"`
	AccessScope      MediaAccessScope      `gorm:"size:16;not null" json:"access_scope"`
	ChecksumSHA256   *string               `gorm:"size:64" json:"checksum_sha256,omitempty"`
	UploadedAt       *time.Time            `json:"uploaded_at,omitempty"`
	ProcessedAt      *time.Time            `json:"processed_at,omitempty"`
	ExpiresAt        time.Time             `json:"expires_at"`
	AttemptCount     int                   `json:"-"`
	NextAttemptAt    time.Time             `json:"-"`
	LockedAt         *time.Time            `json:"-"`
	LockedBy         *string               `json:"-"`
	LastError        *string               `json:"-"`
	Variants         []MediaVariant        `gorm:"foreignKey:MediaAssetID" json:"variants,omitempty"`
}

type MediaVariant struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MediaAssetID uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	Name         string    `gorm:"size:32;not null" json:"name"`
	ObjectKey    string    `gorm:"size:512;not null;uniqueIndex" json:"-"`
	MIMEType     string    `gorm:"size:64;not null" json:"mime_type"`
	ByteSize     int64     `gorm:"not null" json:"byte_size"`
	Width        int       `gorm:"not null" json:"width"`
	Height       int       `gorm:"not null" json:"height"`
	CreatedAt    time.Time `gorm:"not null;autoCreateTime" json:"created_at"`
}

func (MediaAsset) TableName() string   { return "media_assets" }
func (MediaVariant) TableName() string { return "media_variants" }
