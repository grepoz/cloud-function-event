# Cloud Function Event Service

This project implements a Google Cloud Function (Gen 2) in Go for managing events in Firestore.

## 🚨 Fix "Missing `go.sum`" Error

If you see errors like missing `go.sum` entry, it means dependency checksums haven't been generated. Run this in your terminal root:

```go mod tidy```


Or if you have `make` installed:

```make tidy```


## Project Structure

* internal/domain: Data models and DTOs.
* internal/repository: Firestore interactions (Filtering, Sorting).
* internal/service: Business logic.
* internal/transport: HTTP handling and Brotli compression.
* function.go: Cloud Function entry point.

## Testing

Run the unit tests (ensure you run tidy first):

```go test ./...```


## Deployment

To deploy to Google Cloud:

```

```

