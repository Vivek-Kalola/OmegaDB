package db

import (
	"bytes"
	"sync"
	"testing"

	"OmegaDB/schema"
)

func TestCollectionCRUD(t *testing.T) {
	c := NewCollection(nil)

	doc1 := []byte(`{"user": {"id": "123", "email": "test@test.com"}}`)

	// Insert
	id, err := c.Insert(doc1)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if id == "" {
		t.Fatal("Expected a UUID, got empty string")
	}

	// Get
	doc, err := c.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(doc.Raw, doc1) {
		t.Fatal("Retrieved document doesn't match inserted data")
	}

	// Delete
	err = c.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify Delete
	_, err = c.Get(id)
	if err == nil {
		t.Fatal("Expected error getting deleted document")
	}
}

func TestCollectionSchemaEnforcement(t *testing.T) {
	s := &schema.Schema{
		Root: &schema.Field{
			Type: schema.TypeObject,
			Properties: map[string]*schema.Field{
				"required_field": {Type: schema.TypeString, Required: true},
			},
		},
	}
	c := NewCollection(s)

	validDoc := []byte(`{"required_field": "hello"}`)
	_, err := c.Insert(validDoc)
	if err != nil {
		t.Fatalf("Expected valid doc to insert, got error: %v", err)
	}

	invalidDoc := []byte(`{"other_field": "hello"}`)
	_, err = c.Insert(invalidDoc)
	if err == nil {
		t.Fatal("Expected invalid doc to be rejected, but it succeeded")
	}
}

func TestCollectionIndexing(t *testing.T) {
	c := NewCollection(nil)
	c.CreateIndex("user.email")

	doc1 := []byte(`{"user": {"email": "alice@test.com"}}`)
	doc2 := []byte(`{"user": {"email": "bob@test.com"}}`)
	doc3 := []byte(`{"user": {"email": "alice@test.com"}}`) // Duplicate email

	id1, _ := c.Insert(doc1)
	id2, _ := c.Insert(doc2)
	id3, _ := c.Insert(doc3)

	// Lookup Alice
	ids, err := c.FindByExactMatch("user.email", []byte(`"alice@test.com"`))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("Expected 2 documents for alice, found %d", len(ids))
	}

	// Ensure both IDs are returned
	found1, found3 := false, false
	for _, id := range ids {
		if id == id1 {
			found1 = true
		}
		if id == id3 {
			found3 = true
		}
	}
	if !found1 || !found3 {
		t.Fatal("Missing expected IDs in index lookup")
	}

	// Lookup Bob
	ids, _ = c.FindByExactMatch("user.email", []byte(`"bob@test.com"`))
	if len(ids) != 1 || ids[0] != id2 {
		t.Fatalf("Expected only bob's ID")
	}

	// Delete Bob and verify index update
	c.Delete(id2)
	ids, _ = c.FindByExactMatch("user.email", []byte(`"bob@test.com"`))
	if len(ids) != 0 {
		t.Fatalf("Expected 0 documents for bob after deletion, found %d", len(ids))
	}
}

func TestCollectionConcurrency(t *testing.T) {
	c := NewCollection(nil)
	c.CreateIndex("type")

	var wg sync.WaitGroup
	numRoutines := 100
	numInserts := 10

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numInserts; j++ {
				data := []byte(`{"type": "concurrent"}`)
				c.Insert(data)
			}
		}()
	}

	// Read concurrently while inserting
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.FindByExactMatch("type", []byte(`"concurrent"`))
		}()
	}

	wg.Wait()

	ids, _ := c.FindByExactMatch("type", []byte(`"concurrent"`))
	if len(ids) != numRoutines*numInserts {
		t.Fatalf("Expected %d documents, found %d", numRoutines*numInserts, len(ids))
	}
}

func TestCollectionExtract(t *testing.T) {
	c := NewCollection(nil)

	data := []byte(`{"user": {"name": "Alice", "scores": [10, 20, 30]}}`)
	id, err := c.Insert(data)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Extract a nested scalar
	vals, err := c.Extract(id, "user.name")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(vals) != 1 || string(vals[0]) != `"Alice"` {
		t.Fatalf("Expected '\"Alice\"', got %v", vals)
	}

	// Extract via array index
	vals, err = c.Extract(id, "user.scores[2]")
	if err != nil {
		t.Fatalf("Extract with index failed: %v", err)
	}
	if len(vals) != 1 || string(vals[0]) != "30" {
		t.Fatalf("Expected '30', got %v", vals)
	}

	// Extract all array elements via wildcard
	vals, err = c.Extract(id, "user.scores[*]")
	if err != nil {
		t.Fatalf("Extract with wildcard failed: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(vals))
	}
}

func TestCollectionFindWithTableScan(t *testing.T) {
	c := NewCollection(nil) // No index created

	c.Insert([]byte(`{"status": "active"}`))
	c.Insert([]byte(`{"status": "inactive"}`))
	c.Insert([]byte(`{"status": "active"}`))

	// Should fall back to O(n) table scan
	ids, err := c.Find("status", []byte(`"active"`))
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("Expected 2 active docs, got %d", len(ids))
	}
}

func TestCollectionFindWithIndex(t *testing.T) {
	c := NewCollection(nil)
	c.CreateIndex("status") // Has index – should use O(1) path

	c.Insert([]byte(`{"status": "active"}`))
	c.Insert([]byte(`{"status": "inactive"}`))
	c.Insert([]byte(`{"status": "active"}`))

	ids, err := c.Find("status", []byte(`"active"`))
	if err != nil {
		t.Fatalf("Find (indexed) failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("Expected 2 active docs, got %d", len(ids))
	}
}
