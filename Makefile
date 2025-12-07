# Makefile to automate common Go tasks

.PHONY: tidy test run deploy

GOOGLE_CLOUD_PROJECT = local-project-id # bibently-firebase
FIRESTORE_ADMIN_UID = admin_user_xyz_123_secret_id

# Generates the go.sum file and removes unused dependencies
tidy:
	go mod tidy

# Runs all unit tests in the project
test: tidy
	go test ./... -v

# Uruchamia testy integracyjne (wymaga uruchomionego emulatora w innym terminalu)
test-integration: tidy
	FIRESTORE_EMULATOR_HOST="localhost:8080" GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) go test ./test/... -v -count=1

rules:
	@echo "Generating firestore.rules..."
	sed "s/YOUR_ADMIN_UID_HERE/$(FIRESTORE_ADMIN_UID)/g" firestore.rules.template > firestore.rules

# start firestore emulator
start-emulator:
	firebase emulators:start --only firestore --project=$(GOOGLE_CLOUD_PROJECT)

# Helper to run the function locally with emulator
run: tidy
	FIRESTORE_EMULATOR_HOST="localhost:8080" GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) FUNCTION_TARGET=EventFunction LOCAL_ONLY=true go run cmd/main.go

run-real: tidy
	GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) FUNCTION_TARGET=EventFunction LOCAL_ONLY=true FIRESTORE_DATABASE_ID="bibently-store" go run cmd/main.go

swagger:
	swag init -g function.go --output docs

#  to debug run `Debug local function` configuration and go: http://127.0.0.1:5000/swagger/index.html

# Deploy to Google Cloud Functions (Gen 2)
deploy: tidy rules
	gcloud functions deploy event-function \
	--gen2 \
	--runtime=go122 \
	--region=us-central1 \
	--source=. \
	--entry-point=$(FUNCTION_TARGET) \
	--trigger-http \
	--allow-unauthenticated \
#	--set-env-vars=$(shell grep -v '^#' .env | xargs | tr ' ' ',')

	# Deploy the generated rules to Firestore
	firebase deploy --only firestore:rules