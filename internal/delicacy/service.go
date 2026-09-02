package delicacy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"go.uber.org/zap"
)

type Service struct {
	log          *zap.SugaredLogger
	delicacyRepo *Repository
}

func NewDelicacyService(log *zap.SugaredLogger, delicacyRepo *Repository) *Service {
	return &Service{
		log:          log,
		delicacyRepo: delicacyRepo,
	}
}

func (s *Service) CreateDelicacy(ctx context.Context, req *CreateDelicacyRequest, userID *uuid.UUID) (*domain.Delicacy, error) {
	name := req.Name
	desc := req.Description
	var thumbnailUrls []string

	existingDelicacy, err := s.delicacyRepo.GetDelicacyByName(ctx, name)
	if err != nil {
		return nil, apperrors.Internal(s.log, "get delicacy", err, "user_id", userID)
	}
	if existingDelicacy != nil {
		return nil, apperrors.New("DELICACY_ALREADY_EXISTS", fmt.Sprintf("%s has already been created. Try a different name.", existingDelicacy.Name), http.StatusConflict)
	}
	if len(req.ThumbnaiUrls) != 0 {
		thumbnailUrls = req.ThumbnaiUrls
	}
	delicacy := &domain.Delicacy{
		Name:          name,
		Description:   desc,
		ThumbnailURLs: thumbnailUrls,
		CreatedBy:     userID,
	}
	err = s.delicacyRepo.CreateDelicacy(ctx, delicacy)
	if err != nil {
		return nil, apperrors.Internal(s.log, "create delicacy", err, "user_id", userID)
	}

	return delicacy, nil
}
func (s *Service) GetDelicay()     {}
func (s *Service) UpdateDelicacy() {}
func (s *Service) DeleteDelicacy() {}

