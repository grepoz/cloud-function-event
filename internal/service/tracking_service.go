package service

import (
	"bibently/backend/internal/domain"
	"bibently/backend/internal/repository"
	"context"
	"time"

	"github.com/google/uuid"
)

type TrackingService interface {
	TrackEvent(ctx context.Context, event *domain.TrackingEvent) error
	GetAllTracking(ctx context.Context) ([]domain.TrackingEvent, error)
}

type trackingService struct {
	repo repository.TrackingRepository
}

func NewTrackingService(repo repository.TrackingRepository) TrackingService {
	return &trackingService{repo: repo}
}

func (s *trackingService) TrackEvent(ctx context.Context, event *domain.TrackingEvent) error {
	if event.Id == "" {
		event.Id = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	// Basic validation
	if event.Action == "" {
		return domain.ErrValidation("action is required")
	}
	return s.repo.SaveTracking(ctx, event)
}

func (s *trackingService) GetAllTracking(ctx context.Context) ([]domain.TrackingEvent, error) {
	return s.repo.ListTracking(ctx)
}
