package wal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWALAppendAndReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wal")

	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open WAL: %v", err)
	}

	entries := []WALEntry{
		{Op: OpInsert, ID: "id-1", Data: []byte(`{"name":"alice"}`)},
		{Op: OpInsert, ID: "id-2", Data: []byte(`{"name":"bob"}`)},
		{Op: OpDelete, ID: "id-1", Data: nil},
		{Op: OpUpdate, ID: "id-2", Data: []byte(`{"name":"bob-updated"}`)},
	}

	for _, e := range entries {
		if err := w.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	w.Close()

	// Reopen and replay
	w2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen WAL: %v", err)
	}
	defer w2.Close()

	var replayed []WALEntry
	if err := w2.Replay(func(e WALEntry) {
		replayed = append(replayed, e)
	}); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if len(replayed) != len(entries) {
		t.Fatalf("Expected %d entries, got %d", len(entries), len(replayed))
	}
	for i, e := range entries {
		if replayed[i].Op != e.Op || replayed[i].ID != e.ID {
			t.Fatalf("Entry %d mismatch: got op=%c id=%s", i, replayed[i].Op, replayed[i].ID)
		}
	}
}

func TestWALTruncate(t *testing.T) {
	dir := t.TempDir()
	w, _ := Open(filepath.Join(dir, "test.wal"))

	w.Append(WALEntry{Op: OpInsert, ID: "x", Data: []byte(`{}`)})
	w.Truncate()

	var count int
	w.Replay(func(WALEntry) { count++ })
	if count != 0 {
		t.Fatalf("Expected 0 entries after truncate, got %d", count)
	}
	w.Close()
}

func TestWALCrashTolerance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crash.wal")

	w, _ := Open(path)
	w.Append(WALEntry{Op: OpInsert, ID: "ok", Data: []byte(`{"x":1}`)})
	w.Close()

	// Simulate crash by appending a truncated entry directly
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.Write([]byte{byte(OpInsert), 0x05, 0x00, 0x00, 0x00}) // id_len=5 but no more data
	f.Close()

	// Replay should recover only the valid entry
	w2, _ := Open(path)
	defer w2.Close()

	var count int
	w2.Replay(func(WALEntry) { count++ })
	if count != 1 {
		t.Fatalf("Expected 1 valid entry after crash tolerance, got %d", count)
	}
}
