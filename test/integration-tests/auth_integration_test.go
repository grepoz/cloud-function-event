package integration_tests

import (
	"bibently.com/backend/internal/repository"
	"bibently.com/backend/internal/service"
	"bibently.com/backend/internal/transport"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

const TestAdminUID = "admin_user_xyz_123_secret_id"
const TestProjectID = "local-project-id"

// generateValidEmulatorToken creates a fresh, unsigned JWT for the emulator
func generateValidEmulatorToken(uid, projectID string) string {
	header := `{"alg":"none","typ":"JWT"}`
	now := time.Now().Unix()

	payload := map[string]interface{}{
		"iss":       "https://securetoken.google.com/" + projectID,
		"aud":       projectID,
		"auth_time": now,
		"user_id":   uid,
		"sub":       uid,
		"iat":       now,
		"exp":       now + 3600, // Expires in 1 hour
		"email":     uid + "@test.local",
	}

	pBytes, _ := json.Marshal(payload)
	enc := base64.RawURLEncoding
	// Important: The trailing dot indicates an empty signature
	return enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(pBytes) + "."
}

// ensureUserExists creates the user in the Auth Emulator if missing
func ensureUserExists(ctx context.Context, client *auth.Client, uid string) error {
	_, err := client.GetUser(ctx, uid)
	if err == nil {
		return nil // User exists
	}

	params := (&auth.UserToCreate{}).
		UID(uid).
		Email(uid + "@test.local").
		EmailVerified(true).
		Password("password123").
		DisplayName("Integration Test User " + uid)

	_, err = client.CreateUser(ctx, params)
	return err
}

func setupAuthIntegration(t *testing.T) (http.Handler, *firestore.Client) {
	t.Helper()

	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("Skipping auth integration test: Emulator not running")
	}

	ctx := context.Background()

	// 2. Initialize Firestore
	client, err := firestore.NewClient(ctx, TestProjectID)
	if err != nil {
		t.Fatalf("Failed to create firestore client: %v", err)
	}

	// 3. Initialize Firebase App & Auth
	conf := &firebase.Config{ProjectID: TestProjectID}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		t.Fatalf("Failed to init firebase app: %v", err)
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("Failed to get auth client: %v", err)
	}

	// 4. Create Admin User in Emulator
	if err := ensureUserExists(ctx, authClient, TestAdminUID); err != nil {
		t.Fatalf("Failed to ensure admin user exists: %v", err)
	}

	// 5. Build Application Stack
	eventRepo := repository.NewEventRepository(client)
	trackingRepo := repository.NewTrackingRepository(client)
	eventSvc := service.NewEventService(eventRepo)
	trackingSvc := service.NewTrackingService(trackingRepo)

	router := transport.NewRouter(eventSvc, trackingSvc)
	protectedHandler := transport.WithAuthProtection(router, authClient)

	return protectedHandler, client
}

func TestAuth_Strict_Blocking(t *testing.T) {
	handler, client := setupAuthIntegration(t)
	t.Cleanup(func() {
		err := client.Close()
		if err != nil {
			return
		}
	})

	t.Run("Allow_Unauthenticated_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("Block_Unauthenticated_Write", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"event_name": "Hack"}`))
		req := httptest.NewRequest(http.MethodPost, "/events/", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Unauthorized, got %d", w.Code)
		}
	})
}

// TestAuth_Authenticated_Access: Verifies Role-Based Access Control
func TestAuth_Authenticated_Access(t *testing.T) {
	handler, client := setupAuthIntegration(t)
	t.Cleanup(func() {
		err := client.Close()
		if err != nil {
			return
		}
	})

	// Generate tokens
	adminToken := generateValidEmulatorToken(TestAdminUID, TestProjectID)
	adminAuthHeader := "Bearer " + adminToken

	t.Run("Allow_Admin_Write", func(t *testing.T) {
		// Valid Event Payload
		bodyStr := `{"event_name": "Auth Test Event", "city": "Warsaw", "type": "concert", "price": 100, "start_time": "2024-12-31T20:00:00Z"}`
		req := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(bodyStr)))
		req.Header.Set("Authorization", adminAuthHeader)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201 Created for Admin, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Block_NonAdmin_Write", func(t *testing.T) {
		// Scenario: A regular user (valid token, but wrong UID) tries to write
		regularToken := generateValidEmulatorToken("regular_user", TestProjectID)
		regularHeader := "Bearer " + regularToken

		bodyStr := `{"event_name": "Illegal Event", "city": "Nowhere", "type": "concert", "price": 0, "start_time": "2024-12-31T20:00:00Z"}`
		req := httptest.NewRequest(http.MethodPost, "/events/", bytes.NewReader([]byte(bodyStr)))
		req.Header.Set("Authorization", regularHeader)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Expect 403 Forbidden because they are NOT the admin
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for Non-Admin, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Allow_Authenticated_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		req.Header.Set("Authorization", adminAuthHeader)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestAuth_Guest_Mode(t *testing.T) {
	handler, client := setupAuthIntegration(t)
	t.Cleanup(func() {
		err := client.Close()
		if err != nil {
			return
		}
	})

	t.Run("Allow_Guest_Read", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for guest read, got %d", w.Code)
		}
		if w.Header().Get("X-Access-Type") != "Public-Preview" {
			t.Errorf("Expected X-Access-Type header, got empty")
		}
	})

	t.Run("Block_Guest_Write", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"event_name": "Hack"}`))
		req := httptest.NewRequest(http.MethodPost, "/events/", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Unauthorized for guest write, got %d", w.Code)
		}
	})
}

func TestAuth_Guest_Mode_Restricted(t *testing.T) {
	handler, client := setupAuthIntegration(t)
	t.Cleanup(func() {
		_ = client.Close()
	})

	// 1. Events should be ALLOWED
	t.Run("Allow_Guest_Events", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for guest events, got %d", w.Code)
		}
	})

	// 2. Tracking should be BLOCKED (even if publicRead is true for events)
	t.Run("Block_Guest_Tracking", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tracking/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for guest tracking, got %d", w.Code)
		}
	})
}
