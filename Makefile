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
unit-test: tidy
	go test ./test/unit-tests/... -v

# Uruchamia testy integracyjne (wymaga uruchomionego emulatora w innym terminalu)
test-integration: tidy
	FIRESTORE_EMULATOR_HOST=$(FIRESTORE_EMULATOR_HOST) \
	FIREBASE_AUTH_EMULATOR_HOST=$(FIREBASE_AUTH_EMULATOR_HOST) \
	FIRESTORE_DATABASE_ID=$(FIRESTORE_DATABASE_ID) \
	GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) \
	FIRESTORE_ADMIN_UID=$(FIRESTORE_ADMIN_UID) \
	go test ./test/integration-tests/... -v -count=1

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
	FIREBASE_AUTH_EMULATOR_HOST=$(FIREBASE_AUTH_EMULATOR_HOST) FIRESTORE_EMULATOR_HOST=$(FIRESTORE_EMULATOR_HOST) FIRESTORE_DATABASE_ID=$(FIRESTORE_DATABASE_ID) GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) FUNCTION_TARGET=BibentlyFunctions LOCAL_ONLY=true go run cmd/main.go

run-real: tidy
	GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT) FUNCTION_TARGET=BibentlyFunctions LOCAL_ONLY=true FIRESTORE_DATABASE_ID="bibently-store" go run cmd/main.go

swagger:
	swag init -g function.go --output docs

#  to debug run `Debug local function` configuration and go: http://127.0.0.1:3000/swagger/index.html


deploy:
	gcloud functions deploy bibently-functions \
	--flags-file=deploy-config.yaml \
	--service-account=$(FUNCTION_SERVICE_ACCOUNT) \
	--update-env-vars=FIRESTORE_ADMIN_UID=$(FIRESTORE_ADMIN_UID),GOOGLE_CLOUD_PROJECT=$(GOOGLE_CLOUD_PROJECT)

deploy-firebase: rules
	firebase deploy --only firestore

# can set all env vars from .env file
#	--set-env-vars=$(shell grep -v '^#' .env | xargs | tr ' ' ',')

# check the service account used by the function
#gcloud functions describe $(FUNCTION_TARGET) \
#    --region=europe-west1 \
#    --gen2 \
#    --format="value(serviceConfig.serviceAccountEmail)"