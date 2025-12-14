package service

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/repository"
	"context"
	"time"

	"github.com/google/uuid"
)

type EventService interface {
	CreateEvent(ctx context.Context, event *domain.Event) error
	UpdateEvent(ctx context.Context, id string, updates map[string]interface{}) error
	GetEvent(ctx context.Context, id string) (*domain.Event, error)
	DeleteEvent(ctx context.Context, id string) error
	ListEvents(ctx context.Context, request domain.SearchRequest) ([]domain.Event, string, error)
	BatchCreateEvents(ctx context.Context, events []*domain.Event) error
}

type eventService struct {
	repo repository.EventRepository
}

func NewEventService(repo repository.EventRepository) EventService {
	return &eventService{repo: repo}
}

func (s *eventService) CreateEvent(ctx context.Context, event *domain.Event) error {
	if event.Id == "" {
		event.Id = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.EventName == "" {
		return domain.ErrValidation("event name is required")
	}
	return s.repo.Save(ctx, event)
}

func (s *eventService) UpdateEvent(ctx context.Context, id string, updates map[string]interface{}) error {
	if id == "" {
		return domain.ErrValidation("id is required for update")
	}
	if len(updates) == 0 {
		return domain.ErrValidation("no fields to update")
	}

	// remove "id" from updates map if present to prevent primary key tampering
	delete(updates, "id")

	return s.repo.Update(ctx, id, updates)
}

func (s *eventService) GetEvent(ctx context.Context, id string) (*domain.Event, error) {
	if id == "" {
		return nil, domain.ErrValidation("id is required")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *eventService) DeleteEvent(ctx context.Context, id string) error {
	if id == "" {
		return domain.ErrValidation("id is required")
	}
	return s.repo.Delete(ctx, id)
}

func (s *eventService) ListEvents(ctx context.Context, req domain.SearchRequest) ([]domain.Event, string, error) {
	if req.Sorting.PageSize > 100 {
		req.Sorting.PageSize = 100
	}
	return s.repo.List(ctx, req)
}

func (s *eventService) BatchCreateEvents(ctx context.Context, events []*domain.Event) error {
	if len(events) == 0 {
		return domain.ErrValidation("no events to create")
	}

	now := time.Now().UTC()
	for _, event := range events {
		if event.Id == "" {
			event.Id = uuid.New().String()
		}
		if event.CreatedAt.IsZero() {
			event.CreatedAt = now
		}
		if event.EventName == "" {
			return domain.ErrValidation("event name is required for all items")
		}
	}
	return s.repo.BatchSave(ctx, events)
}
