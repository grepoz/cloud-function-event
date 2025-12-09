# Makefile to automate common Go tasks

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: tidy test run deploy rules

# Generates the go.sum file and removes unused dependencies
tidy:
	go mod tidy

# Runs all unit tests in the project
test: tidy
	go test ./... -v

# Uruchamia testy integracyjne (wymaga uruchomionego emulatora w innym terminalu)
test-integration: tidy
	FIRESTORE_EMULATOR_HOST=$(FIRESTORE_EMULATOR_HOST) \
	FIRESTORE_DATABASE_ID=$(FIRESTORE_DATABASE_ID) \
	GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) \
	FIRESTORE_ADMIN_UID=$(FIRESTORE_ADMIN_UID) \
	go test ./test/... -v -count=1

rules:
	@echo "Generating firestore.rules..."
	# Use chained sed to replace both UID and the dynamic database ID
	sed -e "s/YOUR_ADMIN_UID_HERE/$(FIRESTORE_ADMIN_UID)/g" \
	    -e "s/{database}/$(FIRESTORE_DATABASE_ID)/g" firestore.rules.template > firestore.rules

generate-fake-token:
	go run ./cmd/generate_fake_auth_token_for_emulator.go

# start firestore emulator
start-emulators: rules
	FIRESTORE_DATABASE_ID=$(FIRESTORE_DATABASE_ID) firebase emulators:start --only firestore,auth --project=$(GOOGLE_CLOUD_PROJECT)

# Helper to run the function locally with emulator
run: tidy
	FIREBASE_AUTH_EMULATOR_HOST=$(FIREBASE_AUTH_EMULATOR_HOST) FIRESTORE_EMULATOR_HOST=$(FIRESTORE_EMULATOR_HOST) FIRESTORE_DATABASE_ID=$(FIRESTORE_DATABASE_ID) GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) FUNCTION_TARGET=EventFunction LOCAL_ONLY=true go run cmd/main.go

run-real: tidy
	GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) FUNCTION_TARGET=EventFunction LOCAL_ONLY=true FIRESTORE_DATABASE_ID="bibently-store" go run cmd/main.go

swagger:
	swag init -g function.go --output docs

#  to debug run `Debug local function` configuration and go: http://127.0.0.1:5000/swagger/index.html

# Deploy to Google Cloud Functions (Gen 2)
deploy: tidy rules
	gcloud functions deploy event-function \
	--gen2 \
	--runtime=go125 \
	--region=europe-west1 \
	--source=. \
	--entry-point=$(FUNCTION_TARGET) \
	--trigger-http \
	--allow-unauthenticated \
	--set-env-vars APP_ENV=production,CORS_ALLOWED_ORIGIN=* \
#	--set-env-vars=$(shell grep -v '^#' .env | xargs | tr ' ' ',')

	# Deploy the generated rules to Firestore
	firebase deploy --only firestore:rules