package test

import (
	"bibently/backend/internal/domain"
	"context"
)

// MockRepository manually implements Repository for testing
type MockRepository struct {
	SaveFunc      func(ctx context.Context, event *domain.Event) error
	BatchSaveFunc func(ctx context.Context, events []*domain.Event) error
	UpdateFunc    func(ctx context.Context, id string, updates map[string]interface{}) error
	GetByIDFunc   func(ctx context.Context, id string) (*domain.Event, error)
	DeleteFunc    func(ctx context.Context, id string) error
	ListFunc      func(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error)
}

func (m *MockRepository) Save(ctx context.Context, event *domain.Event) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, event)
	}
	return nil
}

func (m *MockRepository) BatchSave(ctx context.Context, events []*domain.Event) error {
	if m.BatchSaveFunc != nil {
		return m.BatchSaveFunc(ctx, events)
	}
	return nil
}

func (m *MockRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, updates)
	}
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockRepository) List(ctx context.Context, search domain.SearchRequest) ([]domain.Event, string, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, search)
	}
	return nil, "", nil
}
