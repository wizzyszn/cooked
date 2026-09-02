package media

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"go.uber.org/zap"
)

const maxUploadBytes int64 = 5 << 20

var allowedMIMEs = map[string]string{"image/jpeg": "jpg", "image/png": "png"}

type AssetRepository interface {
	Create(context.Context, *domain.MediaAsset) error
	Find(context.Context, uuid.UUID) (*domain.MediaAsset, error)
	MarkUploaded(context.Context, uuid.UUID, uuid.UUID, int64, time.Time) error
	Fail(context.Context, uuid.UUID, bool, string, time.Time) error
}

type Service struct {
	repo    AssetRepository
	objects ObjectStore
	log     *zap.SugaredLogger
	now     func() time.Time
}

func NewService(repo AssetRepository, objects ObjectStore, log *zap.SugaredLogger) *Service {
	return &Service{repo: repo, objects: objects, log: log, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Initialize(ctx context.Context, owner uuid.UUID, req InitializeRequest) (*UploadTicket, error) {
	ext, ok := allowedMIMEs[strings.ToLower(strings.TrimSpace(req.MIMEType))]
	if !ok || req.ByteSize <= 0 || req.ByteSize > maxUploadBytes || !validPurpose(req.Purpose) || !validScope(req.AccessScope) {
		return nil, apperrors.WithMessage(apperrors.ErrValidation, "only JPEG/PNG images up to 5 MB and valid purpose/access values are accepted")
	}
	if req.Purpose == domain.MediaPurposeProfileAvatar && req.AccessScope != domain.MediaPublic {
		return nil, apperrors.WithMessage(apperrors.ErrValidation, "profile avatars must be public")
	}
	id := uuid.New()
	now := s.now()
	expiry := now.Add(15 * time.Minute)
	filename := strings.TrimSpace(filepath.Base(req.Filename))
	if filename == "." {
		filename = ""
	}
	key := fmt.Sprintf("original/%s/%s.%s", owner, id, ext)
	asset := &domain.MediaAsset{BaseModel: domain.BaseModel{ID: id}, OwnerID: &owner, Purpose: req.Purpose, ObjectKey: key, DeclaredMIMEType: req.MIMEType, ByteSize: &req.ByteSize, ProcessingStatus: domain.MediaAwaitingUpload, ModerationStatus: domain.MediaModerationPending, AccessScope: req.AccessScope, ExpiresAt: expiry, NextAttemptAt: now}
	if filename != "" {
		asset.OriginalFilename = &filename
	}
	if err := s.repo.Create(ctx, asset); err != nil {
		return nil, apperrors.Internal(s.log, "create media asset", err)
	}
	u, err := s.objects.PresignPut(ctx, key, req.MIMEType, 15*time.Minute)
	if err != nil {
		return nil, apperrors.Internal(s.log, "sign media upload", err)
	}
	return &UploadTicket{AssetID: id, UploadURL: u.String(), ExpiresAt: expiry, RequiredHeaders: map[string]string{"Content-Type": req.MIMEType}}, nil
}
func (s *Service) CompleteUpload(ctx context.Context, owner, id uuid.UUID) (*AssetResponse, error) {
	asset, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, apperrors.Internal(s.log, "find media", err)
	}
	if asset == nil || asset.OwnerID == nil || *asset.OwnerID != owner {
		return nil, apperrors.ErrNotFound
	}
	if asset.ProcessingStatus != domain.MediaAwaitingUpload {
		return s.project(ctx, asset, owner)
	}
	info, err := s.objects.Stat(ctx, asset.ObjectKey)
	if err != nil {
		return nil, apperrors.WithMessage(apperrors.ErrBadRequest, "uploaded object was not found")
	}
	if info.Size <= 0 || info.Size > maxUploadBytes {
		_ = s.objects.Delete(ctx, asset.ObjectKey)
		_ = s.repo.Fail(ctx, id, false, "object exceeds 5 MB", s.now())
		return nil, apperrors.WithMessage(apperrors.ErrValidation, "uploaded object exceeds 5 MB")
	}
	if asset.ByteSize == nil || info.Size != *asset.ByteSize {
		_ = s.objects.Delete(ctx, asset.ObjectKey)
		_ = s.repo.Fail(ctx, id, false, "object size does not match upload declaration", s.now())
		return nil, apperrors.WithMessage(apperrors.ErrValidation, "uploaded object size does not match the declared size")
	}
	if err = s.repo.MarkUploaded(ctx, id, owner, info.Size, s.now()); err != nil {
		return nil, apperrors.ErrConflict
	}
	asset, _ = s.repo.Find(ctx, id)
	return s.project(ctx, asset, owner)
}
func (s *Service) Get(ctx context.Context, requester *uuid.UUID, id uuid.UUID) (*AssetResponse, error) {
	asset, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, apperrors.Internal(s.log, "find media", err)
	}
	if asset == nil {
		return nil, apperrors.ErrNotFound
	}
	return s.project(ctx, asset, requesterValue(requester))
}
func (s *Service) ValidateProfileAvatar(ctx context.Context, owner, id uuid.UUID) error {
	a, err := s.repo.Find(ctx, id)
	if err != nil {
		return apperrors.Internal(s.log, "validate avatar", err)
	}
	if a == nil || a.OwnerID == nil || *a.OwnerID != owner || a.Purpose != domain.MediaPurposeProfileAvatar || a.AccessScope != domain.MediaPublic || a.ProcessingStatus != domain.MediaReady || a.ModerationStatus != domain.MediaModerationApproved {
		return apperrors.WithMessage(apperrors.ErrValidation, "avatar_media_id must reference your ready, approved profile avatar")
	}
	return nil
}
func requesterValue(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}
func (s *Service) project(ctx context.Context, a *domain.MediaAsset, requester uuid.UUID) (*AssetResponse, error) {
	isOwner := a.OwnerID != nil && *a.OwnerID == requester
	if a.ProcessingStatus != domain.MediaReady || a.ModerationStatus != domain.MediaModerationApproved {
		if !isOwner {
			return nil, apperrors.ErrNotFound
		}
	}
	if a.AccessScope == domain.MediaPrivate && !isOwner {
		return nil, apperrors.ErrNotFound
	}
	out := &AssetResponse{ID: a.ID, Purpose: a.Purpose, ProcessingStatus: a.ProcessingStatus, ModerationStatus: a.ModerationStatus, AccessScope: a.AccessScope, MIMEType: a.DecodedMIMEType, ByteSize: a.ByteSize, Width: a.Width, Height: a.Height}
	if a.ProcessingStatus == domain.MediaReady && a.ModerationStatus == domain.MediaModerationApproved {
		u, err := s.objects.PresignGet(ctx, a.ObjectKey, 10*time.Minute)
		if err != nil {
			return nil, apperrors.Internal(s.log, "sign media read", err)
		}
		out.URL = u.String()
		for _, v := range a.Variants {
			vu, e := s.objects.PresignGet(ctx, v.ObjectKey, 10*time.Minute)
			if e != nil {
				return nil, apperrors.Internal(s.log, "sign variant read", e)
			}
			out.Variants = append(out.Variants, VariantResponse{Name: v.Name, MIMEType: v.MIMEType, Width: v.Width, Height: v.Height, URL: vu.String()})
		}
	}
	return out, nil
}
func validPurpose(v domain.MediaPurpose) bool {
	switch v {
	case domain.MediaPurposeProfileAvatar, domain.MediaPurposeDishCover, domain.MediaPurposeRecipeCover, domain.MediaPurposeStepImage, domain.MediaPurposeReviewPhoto, domain.MediaPurposeCookSessionPhoto:
		return true
	}
	return false
}
func validScope(v domain.MediaAccessScope) bool {
	return v == domain.MediaPublic || v == domain.MediaPrivate
}
