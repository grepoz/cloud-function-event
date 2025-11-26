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
		log.Printf("GOOGLE_CLOUD_PROJECT not set, using fallback project ID: %s", projectID)
	}

	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create firestore client: %v", err)
	}
	println("Firestore client initialized")

	// Initialize Repositories (Returns generic container for Events & Tracking)
	event_repository := repository.NewEventRepository(client)
	tracking_repository := repository.NewTrackingRepository(client)

	// Initialize Services
	eventSvc := service.NewEventService(event_repository)
	trackingSvc := service.NewTrackingService(tracking_repository)

	// Initialize Main Router
	router := transport.NewRouter(eventSvc, trackingSvc)

	// Register Cloud Function
	functions.HTTP("EventFunction", func(w http.ResponseWriter, r *http.Request) {
		// 1. Serve Swagger UI at /swagger/
		if strings.HasPrefix(r.URL.Path, "/swagger/") {
			httpSwagger.WrapHandler(w, r)
			return
		}

		// 2. Serve Application Logic (with Compression)
		transport.WithCompression(router).ServeHTTP(w, r)
	})
}

// NOTE: Cloud Functions V2 Entry point is registered via the functions-framework in `init`.
// When deploying, you specify --entry-point=EventFunction
