package db

import "github.com/Vivek-Kalola/omega-db/node"

// Document represents a single JSON record stored in the database.
type Document struct {
	ID   string
	Raw  []byte
	Root *node.Node
}

// Release returns the Document's underlying node tree back to the sync.Pool.
// This must be called when the document is deleted or evicted to prevent memory leaks.
func (d *Document) Release() {
	if d.Root != nil {
		node.Put(d.Root)
		d.Root = nil
	}
}
