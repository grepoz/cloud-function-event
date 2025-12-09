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
	firebase "firebase.google.com/go/v4" // <--- Import Firebase
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"

	_ "cloud-function-event/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Cloud Function Event API
// @version 1.0
// @description API for managing events in Firestore (Google Cloud Function).

// @host 127.0.0.1:5000
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func init() {
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	// 1. Initialize Firestore
	databaseId := os.Getenv("FIRESTORE_DATABASE_ID")
	// Note: In Cloud Functions, projectID is often auto-detected,
	// but we pass it explicitly if needed for local fallback.
	if projectID == "" {
		projectID = "local-project-id"
	}

	fsClient, err := firestore.NewClientWithDatabase(ctx, projectID, databaseId)
	if err != nil {
		log.Fatalf("Failed to create firestore client: %v", err)
	}

	// 2. Initialize Firebase Auth (NEW)
	// We use the same project ID for Firebase Auth
	conf := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		log.Fatalf("error initializing firebase app: %v", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting auth client: %v", err)
	}

	// 3. Initialize Domain Layers
	eventRepo := repository.NewEventRepository(fsClient)
	trackingRepo := repository.NewTrackingRepository(fsClient)

	eventSvc := service.NewEventService(eventRepo)
	trackingSvc := service.NewTrackingService(trackingRepo)

	router := transport.NewRouter(eventSvc, trackingSvc)

	// 4. Configuration
	corsOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")

	isProduction := os.Getenv("APP_ENV") == "production"

	// 5. Register Function
	functions.HTTP("EventFunction", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/swagger/") {
			httpSwagger.Handler(httpSwagger.DeepLinking(false))(w, r)
			return
		}

		// Middleware Chain:
		// CORS -> Security Headers -> Auth -> Compression -> Router

		// 1. Wrap Router with Compression
		handler := transport.WithCompression(router)

		// 2. Wrap with Auth
		handler = transport.WithAuthProtection(handler, authClient)

		// 3. Wrap with Security Headers (NEW)
		handler = transport.WithSecurityHeaders(handler, isProduction)

		// 4. Wrap with CORS (Outer-most layer to handle OPTIONS requests)
		handler = transport.WithCORS(handler, corsOrigin)

		handler.ServeHTTP(w, r)
	})
}
