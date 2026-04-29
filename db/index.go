package db

import (
	"OmegaDB/internal"
	"OmegaDB/node"
	"OmegaDB/query"
)

// Index provides an O(1) Hash map lookup for specific JSON paths.
type Index struct {
	Path string
	// Map stores stringified values mapped to a set of Document IDs.
	// map[ValueString]map[DocID]struct{}
	Map map[string]map[string]struct{}
}

// NewIndex creates a new Hash Index for the specified JSON path.
func NewIndex(path string) *Index {
	return &Index{
		Path: path,
		Map:  make(map[string]map[string]struct{}),
	}
}

// Add extracts the path from the node and adds the Document ID to the index.
func (idx *Index) Add(docID string, root *node.Node) {
	val := extractPathValue(root, idx.Path)
	if val != nil {
		valStr := internal.BytesToString(val)
		if _, ok := idx.Map[valStr]; !ok {
			idx.Map[valStr] = make(map[string]struct{})
		}
		idx.Map[valStr][docID] = struct{}{}
	}
}

// Remove removes the Document ID from the index for the given value.
func (idx *Index) Remove(docID string, root *node.Node) {
	val := extractPathValue(root, idx.Path)
	if val != nil {
		valStr := internal.BytesToString(val)
		if docs, ok := idx.Map[valStr]; ok {
			delete(docs, docID)
			if len(docs) == 0 {
				delete(idx.Map, valStr)
			}
		}
	}
}

// extractPathValue uses the new query engine to compile and extract the value.
// It returns the first matching result.
func extractPathValue(root *node.Node, path string) []byte {
	plan, err := query.Compile(path)
	if err != nil {
		return nil
	}
	results := query.Execute(plan, root)
	if len(results) > 0 {
		return results[0]
	}
	return nil
}
