package service_test

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"context"
	"errors"
	"testing"
)

// MockTrackingRepo
type MockTrackingRepo struct {
	SaveFunc func(ctx context.Context, t *domain.TrackingEvent) error
	ListFunc func(ctx context.Context) ([]domain.TrackingEvent, error)
}

func (m *MockTrackingRepo) SaveTracking(ctx context.Context, t *domain.TrackingEvent) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, t)
	}
	return nil
}

func (m *MockTrackingRepo) ListTracking(ctx context.Context) ([]domain.TrackingEvent, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func TestTrackEvent_Validation(t *testing.T) {
	mockRepo := &MockTrackingRepo{}
	svc := service.NewTrackingService(mockRepo)

	// Case 1: Empty Action
	event := &domain.TrackingEvent{Payload: "something"}
	err := svc.TrackEvent(context.Background(), event)
	if err == nil {
		t.Error("Expected validation error for empty action")
	}
}

func TestTrackEvent_Success(t *testing.T) {
	mockRepo := &MockTrackingRepo{
		SaveFunc: func(ctx context.Context, tr *domain.TrackingEvent) error {
			if tr.ID == "" {
				return errors.New("ID was not generated")
			}
			if tr.CreatedAt.IsZero() {
				return errors.New("CreatedAt was not set")
			}
			return nil
		},
	}
	svc := service.NewTrackingService(mockRepo)

	event := &domain.TrackingEvent{Action: "click"}
	err := svc.TrackEvent(context.Background(), event)
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}
