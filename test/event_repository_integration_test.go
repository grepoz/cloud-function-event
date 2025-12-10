package test

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/repository"
	"context"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
)

// TestEventRepository_List_MultipleFilters verifies that the repository correctly
// handles multiple inequality filters (which previously caused crashes or required specific ordering).
func TestEventRepository_List_MultipleFilters(t *testing.T) {
	// Reuse existing integration setup/teardown logic
	withFirestore(t, func(t *testing.T, _ http.Handler, client *firestore.Client) {
		repo := repository.NewEventRepository(client)
		ctx := context.Background()

		// 1. Prepare Helpers
		floatPtr := func(v float64) *float64 { return &v }
		timePtr := func(t time.Time) *time.Time { return &t }
		baseTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

		// 2. Seed Data
		// We create 4 events covering different combinations of Price and Time
		seedEvents := []domain.Event{
			{
				Id:        "match_perfect",
				EventName: "Perfect Match",
				Price:     100,
				StartTime: baseTime.Add(1 * time.Hour), // Future
				CreatedAt: time.Now(),
			},
			{
				Id:        "fail_price",
				EventName: "Cheap Event",
				Price:     10,                          // < 50 (Fail)
				StartTime: baseTime.Add(1 * time.Hour), // Future (Pass)
				CreatedAt: time.Now(),
			},
			{
				Id:        "fail_time",
				EventName: "Old Event",
				Price:     100,                           // > 50 (Pass)
				StartTime: baseTime.Add(-24 * time.Hour), // Past (Fail)
				CreatedAt: time.Now(),
			},
			{
				Id:        "match_boundary",
				EventName: "Boundary Match",
				Price:     50,       // == 50 (Pass)
				StartTime: baseTime, // == baseTime (Pass)
				CreatedAt: time.Now(),
			},
		}

		for _, e := range seedEvents {
			if err := repo.Save(ctx, &e); err != nil {
				t.Fatalf("Failed to seed event %s: %v", e.Id, err)
			}
		}

		// 3. Define Request with MULTIPLE Inequality Filters
		// Scenario: Filter by MinPrice (50) AND StartDate (baseTime)
		// Limitation Check: We deliberately ask to sort by 'created_at'.
		// The Repo MUST override the primary sort to 'price' or 'start_time' to avoid a crash,
		// while still returning the correct filtered documents.
		req := domain.SearchRequest{
			Filters: domain.FilterRequest{
				MinPrice:  floatPtr(50),
				StartDate: timePtr(baseTime),
			},
			Sorting: domain.SortRequest{
				SortKey:       "created_at", // User intent
				SortDirection: "asc",
				PageSize:      10,
			},
		}

		// 4. Execute List
		results, _, err := repo.List(ctx, req)
		if err != nil {
			t.Fatalf("Repository.List failed with multiple filters: %v", err)
		}

		// 5. Assertions
		if len(results) != 2 {
			t.Errorf("Expected 2 matching events, got %d", len(results))
		}

		foundIds := make(map[string]bool)
		for _, e := range results {
			foundIds[e.Id] = true
		}

		if !foundIds["match_perfect"] {
			t.Error("Expected 'match_perfect' to be in results")
		}
		if !foundIds["match_boundary"] {
			t.Error("Expected 'match_boundary' to be in results")
		}
		if foundIds["fail_price"] {
			t.Error("Found 'fail_price' which should have been filtered out")
		}
		if foundIds["fail_time"] {
			t.Error("Found 'fail_time' which should have been filtered out")
		}
	})
}
