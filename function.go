package function

import (
	"cloud-function-event/internal/repository"
	"cloud-function-event/internal/service"
	"cloud-function-event/internal/transport"
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

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

	// Initialize Layers
	repo := repository.NewFirestoreRepository(client)
	svc := service.NewEventService(repo)

	// Initialize Handler with Logic
	logicHandler := transport.NewHandler(svc)

	// Register Cloud Function
	functions.HTTP("EventFunction", func(w http.ResponseWriter, r *http.Request) {
		// Apply Middleware chain here (e.g., Compression)
		transport.WithCompression(logicHandler).ServeHTTP(w, r)
	})
}

// NOTE: Cloud Functions V2 Entry point is registered via the functions-framework in `init`.
// When deploying, you specify --entry-point=EventFunction
