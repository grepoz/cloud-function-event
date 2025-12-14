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
	"sync"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"

	_ "cloud-function-event/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

// Global variables to hold the initialized state
var (
	functionHandler http.Handler
	initOnce        sync.Once
)

// @host 127.0.0.1:5000
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func init() {
	// Register the entry point, but DO NOT initialize clients here.
	// We defer that to the first request.
	functions.HTTP("BibentlyFunctions", func(w http.ResponseWriter, r *http.Request) {
		// Lazy initialization on first request
		initOnce.Do(func() {
			setupApplication()
		})

		// Delegate to the initialized handler
		functionHandler.ServeHTTP(w, r)
	})
}

// setupApplication contains the logic previously in init()
// It panics on error instead of log.Fatal, allowing the runtime to handle the restart.
func setupApplication() {
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	databaseId := os.Getenv("FIRESTORE_DATABASE_ID")

	if projectID == "" {
		projectID = "local-project-id"
	}

	// 1. Initialize Firestore
	fsClient, err := firestore.NewClientWithDatabase(ctx, projectID, databaseId)
	if err != nil {
		// Use Panic, not Fatal. Panic allows the runtime to catch and restart.
		log.Panicf("Failed to create firestore client: %v", err)
	}

	// 2. Initialize Firebase Auth
	conf := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		log.Panicf("error initializing firebase app: %v", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Panicf("error getting auth client: %v", err)
	}

	// 3. Initialize Domain Layers
	eventRepo := repository.NewEventRepository(fsClient)
	trackingRepo := repository.NewTrackingRepository(fsClient)

	eventSvc := service.NewEventService(eventRepo)
	trackingSvc := service.NewTrackingService(trackingRepo)

	router := transport.NewRouter(eventSvc, trackingSvc)

	// 4. Configuration & Middleware
	corsOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")
	isProduction := os.Getenv("APP_ENV") == "production"

	// Build the middleware chain
	handler := transport.WithCompression(router)
	handler = transport.WithAuthProtection(handler, authClient)
	handler = transport.WithSecurityHeaders(handler, isProduction)
	handler = transport.WithCORS(handler, corsOrigin)

	// Assign to the global handler variable
	// Handle Swagger specific path check inside the main handler wrapper if needed,
	// or wrap it here. For simplicity based on your original code:

	// We create a wrapper to handle Swagger routing vs App routing
	if isProduction == false {
		functionHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/swagger/") {
				httpSwagger.Handler(httpSwagger.DeepLinking(false))(w, r)
				return
			}
			handler.ServeHTTP(w, r)
		})
	}
}
