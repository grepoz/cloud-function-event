package function

import (
	"cloud-function-event/internal/repository"
	"cloud-function-event/internal/service"
	"cloud-function-event/internal/transport"
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"

	_ "cloud-function-event/docs" // This import is required for the side-effect of registering docs

	// Import swagger deps
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Cloud Function Event API
// @version 1.0
// @description API for managing events in Firestore (Google Cloud Function).

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host 127.0.0.1:5000
// @BasePath /
func init() {
	// Initialize Firestore Client
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		// Fallback for local testing
		projectID = "local-project-id"
		log.Printf("GOOGLE_CLOUD_PROJECT not set, using fallback project Id: %s", projectID)
	}

	databaseId := os.Getenv("FIRESTORE_DATABASE_ID")

	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseId)
	if err != nil {
		log.Fatalf("Failed to create firestore client: %v", err)
	}
	println("Firestore client initialized")

	// Initialize Repositories (Returns generic container for Events & Tracking)
	eventRepository := repository.NewEventRepository(client)
	trackingRepository := repository.NewTrackingRepository(client)

	// Initialize Services
	eventSvc := service.NewEventService(eventRepository)
	trackingSvc := service.NewTrackingService(trackingRepository)

	// Initialize Main Router
	router := transport.NewRouter(eventSvc, trackingSvc)

	// 1. Read CORS setting from Env (Setup this in Google Cloud Functions variables)
	corsOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")

	// 2. Read Your New "Control" Switch
	// If "true", guests can read. If "false" (or missing), strict Mode (Option A) is active.
	publicReadAccess := os.Getenv("FIRESTORE_PUBLIC_READ_ACCESS") == "true"

	// Register Cloud Function
	functions.HTTP("EventFunction", func(w http.ResponseWriter, r *http.Request) {
		// Serve Swagger UI (usually strictly GET, but rarely needs CORS if hosted same-origin)
		if strings.HasPrefix(r.URL.Path, "/swagger/") {
			httpSwagger.Handler(
				httpSwagger.DeepLinking(false),
			)(w, r)
			return
		}

		// 3. Construct the Middleware Chain
		// Order: CORS -> Auth Gatekeeper -> Compression -> Router
		// We place Auth inside CORS so preflight checks (OPTIONS) still work.

		// Wrap the router with Compression
		compressedHandler := transport.WithCompression(router)

		// Wrap with your new Auth/Guest Logic
		protectedHandler := transport.WithAuthProtection(compressedHandler, publicReadAccess)

		// Wrap with CORS (Outer layer)
		finalHandler := transport.WithCORS(protectedHandler, corsOrigin)

		finalHandler.ServeHTTP(w, r)
	})
}

// NOTE: Cloud Functions V2 Entry point is registered via the functions-framework in `init`.
// When deploying, you specify --entry-point=EventFunction
