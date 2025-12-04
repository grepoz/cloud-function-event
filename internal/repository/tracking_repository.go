package repository

import (
	"cloud-function-event/internal/domain"
	"cloud.google.com/go/firestore"
	"context"
	"errors"
	"google.golang.org/api/iterator"
)

const CollectionTracking = "tracking"

type TrackingRepository interface {
	SaveTracking(ctx context.Context, tracking *domain.TrackingEvent) error
	ListTracking(ctx context.Context) ([]domain.TrackingEvent, error)
}

type trackingRepo struct {
	client *firestore.Client
}

func NewTrackingRepository(client *firestore.Client) TrackingRepository {
	return &trackingRepo{client: client}
}

func (r *trackingRepo) SaveTracking(ctx context.Context, tracking *domain.TrackingEvent) error {
	if tracking.Id != "" {
		_, err := r.client.Collection(CollectionTracking).Doc(tracking.Id).Set(ctx, tracking)
		return err
	}
	_, _, err := r.client.Collection(CollectionTracking).Add(ctx, tracking)
	return err
}

func (r *trackingRepo) ListTracking(ctx context.Context) ([]domain.TrackingEvent, error) {
	iter := r.client.Collection(CollectionTracking).OrderBy("created_at", firestore.Desc).Documents(ctx)
	defer iter.Stop()

	var tracks []domain.TrackingEvent
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var t domain.TrackingEvent
		if err := doc.DataTo(&t); err != nil {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}
