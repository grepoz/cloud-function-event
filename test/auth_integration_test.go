package test

import (
	"bytes"
	"cloud-function-event/internal/repository"
	"cloud-function-event/internal/service"
	"cloud-function-event/internal/transport"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
)

var adminUID = os.Getenv("FIRESTORE_ADMIN_UID")

// setupAuthIntegration creates a complete stack INCLUDING the Auth Middleware.
func setupAuthIntegration(t *testing.T, publicReadAccess bool) (http.Handler, *firestore.Client) {
	t.Helper()

	// 1. Force Auth Emulator Host for this test process
	// This must be set BEFORE firebase.NewApp is called.
	//_ = os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:9099")
	//_ = os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080")

	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	// 2. Initialize Firestore
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create firestore client: %v", err)
	}

	// 3. Initialize Firebase Auth
	conf := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		t.Fatalf("Failed to init firebase app: %v", err)
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("Failed to get auth client: %v", err)
	}

	// 4. Build Service Stack
	eventRepo := repository.NewEventRepository(client)
	trackingRepo := repository.NewTrackingRepository(client)
	eventSvc := service.NewEventService(eventRepo)
	trackingSvc := service.NewTrackingService(trackingRepo)

	// 5. Build Router & Wrap with Auth
	router := transport.NewRouter(eventSvc, trackingSvc)
	protectedHandler := transport.WithAuthProtection(router, authClient, publicReadAccess)

	return protectedHandler, client
}

// TestAuth_Strict_Blocking verifies that if Public Read is FALSE,
// NO ONE can access the API without a token.
func TestAuth_Strict_Blocking(t *testing.T) {
	handler, client := setupAuthIntegration(t, false)
	// Fix: Register Close FIRST so it runs LAST (LIFO)
	t.Cleanup(func() { client.Close() })

	t.Run("Block_Unauthenticated_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for read, got %d", w.Code)
		}
	})

	t.Run("Block_Unauthenticated_Write", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"event_name": "Hack"}`))
		req := httptest.NewRequest(http.MethodPost, "/events/", body)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for write, got %d", w.Code)
		}
	})
}

// TestAuth_Authenticated_Access verifies that a user with a valid token
// CAN read and write, regardless of public settings.
func TestAuth_Authenticated_Access(t *testing.T) {
	handler, client := setupAuthIntegration(t, false)

	// Fix: Register Close FIRST so it runs LAST (after cleanupFirestore)
	t.Cleanup(func() { client.Close() })
	t.Cleanup(func() { cleanupFirestore(t, client) })

	// Generate a valid Token for the Emulator
	// Ensure "local-project-id" matches what is in setupAuthIntegration
	adminToken := createEmulatorToken(adminUID, "local-project-id")
	bearerHeader := "Bearer " + adminToken

	t.Run("Allow_Authenticated_Write", func(t *testing.T) {
		bodyStr := `{"event_name": "Authorized Concert", "city": "Warsaw", "type": "concert", "price": 100, "start_time": "2024-12-31T20:00:00Z"}`
		req := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(bodyStr)))
		req.Header.Set("Authorization", bearerHeader)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			// If this fails with 401, check if your Auth Emulator is actually running on port 9099
			t.Errorf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Allow_Authenticated_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		req.Header.Set("Authorization", bearerHeader)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", w.Code)
		}
	})
}

func TestAuth_Guest_Mode(t *testing.T) {
	handler, client := setupAuthIntegration(t, true)
	t.Cleanup(func() { client.Close() })

	t.Run("Allow_Guest_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for guest read, got %d", w.Code)
		}
		if w.Header().Get("X-Access-Type") != "Public-Preview" {
			t.Error("Expected X-Access-Type header for guest")
		}
	})

	t.Run("Block_Guest_Write", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"event_name": "Hack"}`))
		req := httptest.NewRequest(http.MethodPost, "/events/", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for guest write, got %d", w.Code)
		}
	})
}
