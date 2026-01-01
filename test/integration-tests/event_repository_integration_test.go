package integration_tests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"bibently.com/backend/internal/domain"
	"bibently.com/backend/internal/repository"

	"cloud.google.com/go/firestore"
)

func TestEventRepository_List_MultipleFilters_RoughMatch(t *testing.T) {
	withFirestore(t, func(t *testing.T, _ http.Handler, client *firestore.Client) {
		// 0. CLEANUP: Delete all existing events before seeding.
		cleanupFirestore(t, client)

		repo := repository.NewEventRepository(client)
		ctx := context.Background()

		// 1. Setup Data Helpers
		floatPtr := func(v float64) *float64 { return &v }
		timePtr := func(t time.Time) *time.Time { return &t }
		baseTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

		// 2. Seed a LARGE dataset (100 Events)
		// Distribution:
		// - 25 Perfect Matches (Price=100, Future)
		// - 25 Fail Price     (Price=10, Future)
		// - 25 Fail Time      (Price=100, Past)
		// - 25 Fail Both      (Price=10, Past)
		expectedMatches := 25

		batch := client.Batch()
		for i := 0; i < expectedMatches; i++ {
			// Match
			batch.Set(client.Collection("events").NewDoc(), &domain.Event{
				Id:        fmt.Sprintf("match_%d", i),
				Name:      "Match",
				Offer:     domain.Offer{Price: 100},
				StartDate: baseTime.Add(time.Hour),
				CreatedAt: time.Now(),
			})
			// Fail Price
			batch.Set(client.Collection("events").NewDoc(), &domain.Event{
				Id:        fmt.Sprintf("fail_price_%d", i),
				Name:      "Fail Price",
				Offer:     domain.Offer{Price: 10},
				StartDate: baseTime.Add(time.Hour),
				CreatedAt: time.Now(),
			})
			// Fail Time
			batch.Set(client.Collection("events").NewDoc(), &domain.Event{
				Id:        fmt.Sprintf("fail_time_%d", i),
				Name:      "Fail Time",
				Offer:     domain.Offer{Price: 100},
				StartDate: baseTime.Add(-24 * time.Hour),
				CreatedAt: time.Now(),
			})
			// Fail Both
			batch.Set(client.Collection("events").NewDoc(), &domain.Event{
				Id:        fmt.Sprintf("fail_both_%d", i),
				Name:      "Fail Both",
				Offer:     domain.Offer{Price: 10},
				StartDate: baseTime.Add(-24 * time.Hour),
				CreatedAt: time.Now(),
			})
		}
		if _, err := batch.Commit(ctx); err != nil {
			t.Fatalf("Batch seed failed: %v", err)
		}

		// 3. Define the Complex Query
		req := domain.SearchRequest{
			Filters: domain.FilterRequest{
				MinPrice:  floatPtr(50),
				StartDate: timePtr(baseTime),
			},
			Sorting: domain.SortRequest{
				SortKey:       "created_at",
				SortDirection: "asc",
				PageSize:      100,
			},
		}

		// 4. Execute
		results, _, err := repo.List(ctx, req)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		count := len(results)
		t.Logf("Query returned %d events (Expected exactly %d)", count, expectedMatches)

		// 5. Assertions
		if count < expectedMatches {
			t.Errorf("Too few results! Got %d, expected at least %d matching events.", count, expectedMatches)
		}

		maxAllowed := expectedMatches
		if count > maxAllowed {
			t.Errorf("Too many results! Got %d. The secondary filter seems ineffective (Max allowed: %d).", count, maxAllowed)
		} else if count > expectedMatches {
			t.Logf("⚠️  Note: Got %d results (expected %d). Emulator returned %d false positives, but filtering IS active.", count, expectedMatches, count-expectedMatches)
		} else {
			t.Log("✅ Perfect match!")
		}
	})
}

func TestEventRepository_List_Keywords(t *testing.T) {
	withFirestore(t, func(t *testing.T, _ http.Handler, client *firestore.Client) {
		repo := repository.NewEventRepository(client)
		ctx := context.Background()

		// 1. Seed Data
		events := []*domain.Event{
			{Id: "1", Name: "Rock Concert", Keywords: []string{"music", "rock", "loud"}, CreatedAt: time.Now()},
			{Id: "2", Name: "Jazz Night", Keywords: []string{"music", "jazz", "chill"}, CreatedAt: time.Now()},
			{Id: "3", Name: "Tech Talk", Keywords: []string{"education", "tech"}, CreatedAt: time.Now()},
		}

		for _, e := range events {
			if _, err := client.Collection("events").Doc(e.Id).Set(ctx, e); err != nil {
				t.Fatalf("Failed to seed event %s: %v", e.Id, err)
			}
		}

		// 2. Test: Search for "rock" (Should find Event 1)
		reqRock := domain.SearchRequest{
			Filters: domain.FilterRequest{Keywords: []string{"rock"}},
			Sorting: domain.SortRequest{PageSize: 10, SortKey: "created_at", SortDirection: "asc"},
		}
		res, _, err := repo.List(ctx, reqRock)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(res) != 1 || res[0].Name != "Rock Concert" {
			t.Errorf("Expected 'Rock Concert', got %v", res)
		}

		// 3. Test: Search for "music" (Should find Event 1 and 2)
		reqMusic := domain.SearchRequest{
			Filters: domain.FilterRequest{Keywords: []string{"music"}},
			Sorting: domain.SortRequest{PageSize: 10, SortKey: "created_at", SortDirection: "asc"},
		}
		res, _, err = repo.List(ctx, reqMusic)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(res) != 2 {
			t.Errorf("Expected 2 music events, got %d", len(res))
		}

		// 4. Test: Search for "tech" OR "jazz" (Should find Event 2 and 3)
		reqMix := domain.SearchRequest{
			Filters: domain.FilterRequest{Keywords: []string{"tech", "jazz"}},
			Sorting: domain.SortRequest{PageSize: 10, SortKey: "created_at", SortDirection: "asc"},
		}
		res, _, err = repo.List(ctx, reqMix)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(res) != 2 {
			t.Errorf("Expected 2 events (Jazz + Tech), got %d", len(res))
		}
	})
}
