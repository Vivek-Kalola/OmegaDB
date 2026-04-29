package store

import (
	"path/filepath"
	"testing"
)

func TestSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.snap")

	docs := []SnapshotDoc{
		{ID: "id-1", Data: []byte(`{"name":"alice","age":30}`)},
		{ID: "id-2", Data: []byte(`{"name":"bob","active":true}`)},
		{ID: "id-3", Data: []byte(`{"nested":{"key":"value"}}`)},
	}

	if err := WriteSnapshot(path, docs); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}

	loaded, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}

	if len(loaded) != len(docs) {
		t.Fatalf("Expected %d docs, got %d", len(docs), len(loaded))
	}

	for i, d := range docs {
		if loaded[i].ID != d.ID {
			t.Fatalf("Doc %d ID mismatch: got %s, want %s", i, loaded[i].ID, d.ID)
		}
		if string(loaded[i].Data) != string(d.Data) {
			t.Fatalf("Doc %d data mismatch", i)
		}
	}
}

func TestSnapshotMissingFile(t *testing.T) {
	docs, err := ReadSnapshot("/nonexistent/path/collection.snap")
	if err != nil {
		t.Fatalf("Expected no error for missing snapshot, got: %v", err)
	}
	if docs != nil {
		t.Fatalf("Expected nil docs for missing snapshot")
	}
}

func TestSnapshotEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.snap")

	if err := WriteSnapshot(path, []SnapshotDoc{}); err != nil {
		t.Fatalf("WriteSnapshot empty: %v", err)
	}

	docs, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("ReadSnapshot empty: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("Expected 0 docs, got %d", len(docs))
	}
}
