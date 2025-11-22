# Makefile to automate common Go tasks

.PHONY: tidy test run deploy

# Generates the go.sum file and removes unused dependencies
tidy:
	go mod tidy

# Runs all unit tests in the project
test: tidy
	go test ./... -v

# start firestore emulator
project_id = local-project-id
start-emulator:
	firebase emulators:start --only firestore --project=$(project_id)

# Helper to run the function locally with emulator
run: tidy
	FIRESTORE_EMULATOR_HOST="localhost:8080" GOOGLE_CLOUD_PROJECT=$(project_id) FUNCTION_TARGET=EventFunction LOCAL_ONLY=true go run cmd/main.go

# Deploy to Google Cloud Functions (Gen 2)
deploy: tidy
	gcloud functions deploy event-function \
	--gen2 \
	--runtime=go121 \
	--region=us-central1 \
	--source=. \
	--entry-point=EventFunction \
	--trigger-http \
	--allow-unauthenticated