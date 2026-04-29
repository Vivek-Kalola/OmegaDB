package db

import (
	"path/filepath"
	"testing"
)

func TestCollectionUpdate(t *testing.T) {
	c := NewCollection(nil)
	c.CreateIndex("status")

	id, _ := c.Insert([]byte(`{"status": "pending"}`))

	// Verify original indexed value
	ids, _ := c.FindByExactMatch("status", []byte(`"pending"`))
	if len(ids) != 1 {
		t.Fatalf("Expected 1 pending doc, got %d", len(ids))
	}

	// Update the document
	if err := c.Update(id, []byte(`{"status": "done"}`)); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Old value must be gone from index
	ids, _ = c.FindByExactMatch("status", []byte(`"pending"`))
	if len(ids) != 0 {
		t.Fatalf("Expected 0 pending docs after update, got %d", len(ids))
	}

	// New value must appear in index
	ids, _ = c.FindByExactMatch("status", []byte(`"done"`))
	if len(ids) != 1 {
		t.Fatalf("Expected 1 done doc, got %d", len(ids))
	}

	// Raw bytes must be updated
	doc, _ := c.Get(id)
	if string(doc.Raw) != `{"status": "done"}` {
		t.Fatalf("Expected updated raw, got: %s", doc.Raw)
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// --- First session: open, insert, close ---
	col, err := OpenCollection(dir, nil)
	if err != nil {
		t.Fatalf("OpenCollection: %v", err)
	}

	id1, _ := col.Insert([]byte(`{"user": "alice", "role": "admin"}`))
	id2, _ := col.Insert([]byte(`{"user": "bob", "role": "viewer"}`))

	if err := col.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// --- Second session: reopen, verify data survived ---
	col2, err := OpenCollection(dir, nil)
	if err != nil {
		t.Fatalf("Reopen OpenCollection: %v", err)
	}
	defer col2.Close()

	doc1, err := col2.Get(id1)
	if err != nil {
		t.Fatalf("Get id1 after reopen: %v", err)
	}
	if string(doc1.Raw) != `{"user": "alice", "role": "admin"}` {
		t.Fatalf("id1 data mismatch: %s", doc1.Raw)
	}

	doc2, err := col2.Get(id2)
	if err != nil {
		t.Fatalf("Get id2 after reopen: %v", err)
	}
	if string(doc2.Raw) != `{"user": "bob", "role": "viewer"}` {
		t.Fatalf("id2 data mismatch: %s", doc2.Raw)
	}
}

func TestWALReplayAfterCrash(t *testing.T) {
	dir := t.TempDir()

	// First session: insert but don't Close() — simulate crash
	col, _ := OpenCollection(dir, nil)
	id, _ := col.Insert([]byte(`{"event": "crash-test"}`))
	// Deliberately NOT calling col.Close() — no snapshot written
	col.wal.Close() // close file handles only, WAL not truncated

	// Second session: must recover from WAL alone
	col2, err := OpenCollection(dir, nil)
	if err != nil {
		t.Fatalf("OpenCollection after crash: %v", err)
	}
	defer col2.Close()

	doc, err := col2.Get(id)
	if err != nil {
		t.Fatalf("Document not recovered from WAL: %v", err)
	}
	if string(doc.Raw) != `{"event": "crash-test"}` {
		t.Fatalf("Recovered data mismatch: %s", doc.Raw)
	}
}

func TestPersistenceWithIndexRebuild(t *testing.T) {
	dir := t.TempDir()

	// Write data
	col, _ := OpenCollection(dir, nil)
	col.Insert([]byte(`{"city": "Mumbai"}`))
	col.Insert([]byte(`{"city": "Bangalore"}`))
	col.Insert([]byte(`{"city": "Mumbai"}`))
	col.Close()

	// Reopen, create index (backfills from persisted docs)
	col2, _ := OpenCollection(dir, nil)
	defer col2.Close()
	col2.CreateIndex("city")

	ids, err := col2.FindByExactMatch("city", []byte(`"Mumbai"`))
	if err != nil {
		t.Fatalf("FindByExactMatch: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("Expected 2 Mumbai docs after persist+rebuild, got %d", len(ids))
	}
}

// Make sure filepath is used (avoid unused import error)
var _ = filepath.Join
