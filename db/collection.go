package db

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"OmegaDB/internal"
	"OmegaDB/node"
	"OmegaDB/parser"
	"OmegaDB/query"
	"OmegaDB/schema"
	"OmegaDB/store"
	"OmegaDB/wal"

	"github.com/google/uuid"
)

// Collection manages a set of documents and their associated indexes.
// Use NewCollection for a transient in-memory collection, or OpenCollection
// for a durable persistent collection backed by WAL + snapshot files.
type Collection struct {
	mu      sync.RWMutex
	docs    map[string]*Document
	indexes map[string]*Index
	schema  *schema.Schema

	// persistence (nil when running in-memory only)
	wal     *wal.WAL
	snapDir string
}

// NewCollection initializes an ephemeral in-memory collection with an optional schema.
func NewCollection(s *schema.Schema) *Collection {
	return &Collection{
		docs:    make(map[string]*Document),
		indexes: make(map[string]*Index),
		schema:  s,
	}
}

// OpenCollection opens (or creates) a durable collection stored in dir.
// On startup it:
//  1. Loads the latest snapshot (if any)
//  2. Replays the WAL to catch up any un-snapshotted writes
//  3. Rebuilds all previously registered indexes from the restored documents
func OpenCollection(dir string, s *schema.Schema) (*Collection, error) {
	c := &Collection{
		docs:    make(map[string]*Document),
		indexes: make(map[string]*Index),
		schema:  s,
		snapDir: dir,
	}

	// 1. Load snapshot
	snapDocs, err := store.ReadSnapshot(filepath.Join(dir, "collection.snap"))
	if err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}
	for _, sd := range snapDocs {
		root, err := parser.Parse(sd.Data)
		if err != nil {
			return nil, fmt.Errorf("re-parsing snapshot doc %s: %w", sd.ID, err)
		}
		c.docs[sd.ID] = &Document{ID: sd.ID, Raw: sd.Data, Root: root}
	}

	// 2. Open and replay WAL
	w, err := wal.Open(filepath.Join(dir, "collection.wal"))
	if err != nil {
		return nil, fmt.Errorf("opening WAL: %w", err)
	}
	c.wal = w

	if err := w.Replay(func(entry wal.WALEntry) {
		switch entry.Op {
		case wal.OpInsert:
			root, err := parser.Parse(entry.Data)
			if err != nil {
				return // skip corrupt entry
			}
			c.docs[entry.ID] = &Document{ID: entry.ID, Raw: entry.Data, Root: root}

		case wal.OpUpdate:
			if old, exists := c.docs[entry.ID]; exists {
				old.Release()
			}
			root, err := parser.Parse(entry.Data)
			if err != nil {
				return
			}
			c.docs[entry.ID] = &Document{ID: entry.ID, Raw: entry.Data, Root: root}

		case wal.OpDelete:
			if doc, exists := c.docs[entry.ID]; exists {
				doc.Release()
				delete(c.docs, entry.ID)
			}
		}
	}); err != nil {
		return nil, fmt.Errorf("replaying WAL: %w", err)
	}

	return c, nil
}

// Close flushes a snapshot and truncates the WAL, then closes file handles.
// Always call Close on a persistent collection when done.
func (c *Collection) Close() error {
	if c.wal == nil {
		return nil // in-memory collection, nothing to do
	}

	c.mu.RLock()
	snapDocs := make([]store.SnapshotDoc, 0, len(c.docs))
	for _, doc := range c.docs {
		snapDocs = append(snapDocs, store.SnapshotDoc{ID: doc.ID, Data: doc.Raw})
	}
	c.mu.RUnlock()

	if err := store.WriteSnapshot(filepath.Join(c.snapDir, "collection.snap"), snapDocs); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}

	if err := c.wal.Truncate(); err != nil {
		return fmt.Errorf("truncating WAL: %w", err)
	}

	return c.wal.Close()
}

// CreateIndex prepares a Hash Index for a given JSON path, backfilling existing documents.
func (c *Collection) CreateIndex(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indexes[path]; !exists {
		idx := NewIndex(path)
		c.indexes[path] = idx
		for id, doc := range c.docs {
			idx.Add(id, doc.Root)
		}
	}
}

// Insert parses, validates, and durably stores a JSON document, returning its UUID.
func (c *Collection) Insert(data []byte) (string, error) {
	root, err := parser.Parse(data)
	if err != nil {
		return "", err
	}

	if c.schema != nil {
		if err := c.schema.Validate(root); err != nil {
			node.Put(root)
			return "", err
		}
	}

	id := uuid.New().String()

	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	// WAL write before in-memory update (durability guarantee)
	if c.wal != nil {
		if err := c.wal.Append(wal.WALEntry{Op: wal.OpInsert, ID: id, Data: dataCopy}); err != nil {
			node.Put(root)
			return "", fmt.Errorf("WAL append: %w", err)
		}
	}

	doc := &Document{ID: id, Raw: dataCopy, Root: root}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.docs[id] = doc
	for _, idx := range c.indexes {
		idx.Add(id, root)
	}

	return id, nil
}

// Update atomically replaces a document's content, keeping the same UUID.
// Indexes are updated and the old AST is returned to the pool.
func (c *Collection) Update(id string, newData []byte) error {
	newRoot, err := parser.Parse(newData)
	if err != nil {
		return err
	}

	if c.schema != nil {
		if err := c.schema.Validate(newRoot); err != nil {
			node.Put(newRoot)
			return err
		}
	}

	dataCopy := make([]byte, len(newData))
	copy(dataCopy, newData)

	// WAL write before in-memory update
	if c.wal != nil {
		if err := c.wal.Append(wal.WALEntry{Op: wal.OpUpdate, ID: id, Data: dataCopy}); err != nil {
			node.Put(newRoot)
			return fmt.Errorf("WAL append: %w", err)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	oldDoc, exists := c.docs[id]
	if !exists {
		node.Put(newRoot)
		return errors.New("document not found")
	}

	// Remove old values from indexes
	for _, idx := range c.indexes {
		idx.Remove(id, oldDoc.Root)
	}

	// Release old AST nodes
	oldDoc.Release()

	// Commit new document
	newDoc := &Document{ID: id, Raw: dataCopy, Root: newRoot}
	c.docs[id] = newDoc

	// Add new values to indexes
	for _, idx := range c.indexes {
		idx.Add(id, newRoot)
	}

	return nil
}

// Get retrieves a document by its UUID.
func (c *Collection) Get(id string) (*Document, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	doc, exists := c.docs[id]
	if !exists {
		return nil, errors.New("document not found")
	}
	return doc, nil
}

// Delete removes a document by UUID and updates all indexes.
func (c *Collection) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	doc, exists := c.docs[id]
	if !exists {
		return errors.New("document not found")
	}

	// WAL write before in-memory update
	if c.wal != nil {
		if err := c.wal.Append(wal.WALEntry{Op: wal.OpDelete, ID: id}); err != nil {
			return fmt.Errorf("WAL append: %w", err)
		}
	}

	for _, idx := range c.indexes {
		idx.Remove(id, doc.Root)
	}
	delete(c.docs, id)
	doc.Release()

	return nil
}

// FindByExactMatch utilizes Hash Indexes to retrieve a list of Document IDs matching the value.
func (c *Collection) FindByExactMatch(path string, value []byte) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	idx, exists := c.indexes[path]
	if !exists {
		return nil, fmt.Errorf("no index exists for path '%s'", path)
	}

	valStr := internal.BytesToString(value)
	docMap, exists := idx.Map[valStr]
	if !exists {
		return []string{}, nil
	}

	ids := make([]string, 0, len(docMap))
	for id := range docMap {
		ids = append(ids, id)
	}
	return ids, nil
}

// Extract fetches one document and runs the query plan, returning all matching raw []byte slices.
func (c *Collection) Extract(docID string, queryStr string) ([][]byte, error) {
	c.mu.RLock()
	doc, exists := c.docs[docID]
	c.mu.RUnlock()

	if !exists {
		return nil, errors.New("document not found")
	}

	plan, err := query.Compile(queryStr)
	if err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}
	return query.Execute(plan, doc.Root), nil
}

// Find returns Document IDs whose value at queryStr matches exactMatch.
// Uses Hash Index O(1) if available, falls back to O(n) table scan.
func (c *Collection) Find(queryStr string, exactMatch []byte) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Fast path: Hash Index
	if idx, ok := c.indexes[queryStr]; ok {
		valStr := internal.BytesToString(exactMatch)
		docMap := idx.Map[valStr]
		ids := make([]string, 0, len(docMap))
		for id := range docMap {
			ids = append(ids, id)
		}
		return ids, nil
	}

	// Slow path: full table scan
	plan, err := query.Compile(queryStr)
	if err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	matchStr := internal.BytesToString(exactMatch)
	var ids []string
	for id, doc := range c.docs {
		for _, r := range query.Execute(plan, doc.Root) {
			if internal.BytesToString(r) == matchStr {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids, nil
}
