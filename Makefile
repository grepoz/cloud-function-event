# Makefile to automate common Go tasks

.PHONY: tidy test run deploy
# Generates the go.sum file and removes unused dependencies
tidy:
	go mod tidy

# Runs all unit tests in the project
test: tidy
	go test ./... -v

# Uruchamia testy integracyjne (wymaga uruchomionego emulatora w innym terminalu)
test-integration: tidy
	FIRESTORE_EMULATOR_HOST="localhost:8080" GOOGLE_CLOUD_PROJECT=$(project_id) go test ./test/... -v -count=1

# start firestore emulator
project_id = local-project-id
start-emulator:
	firebase emulators:start --only firestore --project=$(project_id)

# Helper to run the function locally with emulator
run: tidy
	FIRESTORE_EMULATOR_HOST="localhost:8080" GOOGLE_CLOUD_PROJECT=$(project_id) FUNCTION_TARGET=EventFunction LOCAL_ONLY=true go run cmd/main.go

swagger:
	swag init -g function.go --output docs

#  to debug run `Debug local function` configuration and go: http://127.0.0.1:5000/swagger/index.html

# Deploy to Google Cloud Functions (Gen 2)
#deploy: tidy
#	gcloud functions deploy event-function \
#	--gen2 \
#	--runtime=go121 \
#	--region=us-central1 \
#	--source=. \
#	--entry-point=EventFunction \
#	--trigger-http \
#	--allow-unauthenticated