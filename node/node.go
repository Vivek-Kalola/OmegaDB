package node

import (
	"sync"
)

// NodeType represents the JSON type of the node.
type NodeType byte

const (
	TypeObject NodeType = iota
	TypeArray
	TypeString
	TypeNumber
	TypeBool
	TypeNull
)

// Node represents a single element in the lazily-loaded JSON AST.
type Node struct {
	Type NodeType
	Raw  []byte // Raw slice of the original JSON for lazy parsing

	// Key is the raw slice representing the object key, if this node is a value in an object.
	Key []byte

	Child *Node // Pointer to the first child (for Object and Array)
	Next  *Node // Pointer to the next sibling (for elements in Object and Array)
}

// nodePool manages a pool of *Node to minimize GC allocations.
var nodePool = sync.Pool{
	New: func() interface{} {
		return &Node{}
	},
}

// Get retrieves a clean Node from the pool.
func Get() *Node {
	n := nodePool.Get().(*Node)
	n.Type = TypeNull
	n.Raw = nil
	n.Key = nil
	n.Child = nil
	n.Next = nil
	return n
}

// Put returns a Node and all of its descendants/siblings to the pool.
func Put(n *Node) {
	curr := n
	for curr != nil {
		next := curr.Next
		if curr.Child != nil {
			Put(curr.Child)
		}
		// Clear pointers before putting back, though Get() also zeroes them out
		curr.Child = nil
		curr.Next = nil
		nodePool.Put(curr)
		curr = next
	}
}
