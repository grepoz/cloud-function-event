package integration_tests

import (
	"context"
	"net/http"
	"testing"

	"cloud.google.com/go/firestore"
)

func TestIntegration_VectorSearch(t *testing.T) {
	withFirestore(t, func(t *testing.T, router http.Handler, client *firestore.Client) {
		ctx := context.Background()
		collectionName := "vector_docs"

		// 1. Create documents with vector fields
		// Note: Firestore Vector Search requires a specific vector index to be created first in a real environment.
		// However, for the emulator, it might work without explicit index creation or might require configuration.
		// We will try to rely on the emulator's behavior.

		// Vector data: 3D float32 vectors
		doc1 := map[string]interface{}{
			"name":      "Doc A",
			"embedding": firestore.Vector32{1.0, 2.0, 3.0},
		}
		doc2 := map[string]interface{}{
			"name":      "Doc B",
			"embedding": firestore.Vector32{4.0, 5.0, 6.0},
		}
		doc3 := map[string]interface{}{
			"name":      "Doc C",
			"embedding": firestore.Vector32{1.0, 1.0, 1.0},
		}

		// Add documents
		if _, err := client.Collection(collectionName).Doc("doc1").Set(ctx, doc1); err != nil {
			t.Fatalf("Failed to add doc1: %v", err)
		}
		if _, err := client.Collection(collectionName).Doc("doc2").Set(ctx, doc2); err != nil {
			t.Fatalf("Failed to add doc2: %v", err)
		}
		if _, err := client.Collection(collectionName).Doc("doc3").Set(ctx, doc3); err != nil {
			t.Fatalf("Failed to add doc3: %v", err)
		}

		// verify documents are added
		iter := client.Collection(collectionName).Documents(ctx)
		docs, err := iter.GetAll()
		if err != nil {
			t.Fatalf("Failed to get documents: %v", err)
		}
		if len(docs) != 3 {
			t.Fatalf("Expected 3 documents, got %d", len(docs))
		}

		// 2. Perform Vector Search (FindNearest)
		// We want to find documents nearest to [1.0, 2.0, 3.0] (which is exactly Doc A)
		queryVector := firestore.Vector32{1.0, 2.0, 3.0}

		// FindNearest is the method for vector search
		query := client.Collection(collectionName).FindNearest(
			"embedding",
			queryVector,
			10, // Limit
			firestore.DistanceMeasureEuclidean,
			nil, // Options
		)

		docsIter := query.Documents(ctx)
		results, err := docsIter.GetAll()
		if err != nil {
			// If vector search is not supported or index is missing, this might fail.
			// We want to explicitly check if it works.
			t.Fatalf("Vector search failed (search support might be missing in emulator?): %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Vector search returned no results")
		}

		// The first result should be Doc A because it has distance 0
		firstDoc := results[0]
		name, ok := firstDoc.Data()["name"].(string)
		if !ok || name != "Doc A" {
			t.Errorf("Expected first result to be Doc A, got %v", firstDoc.Data())
		}
	})
}
