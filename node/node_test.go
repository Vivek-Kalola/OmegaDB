package node

import (
	"testing"
)

func TestNodePool(t *testing.T) {
	n := Get()
	if n == nil {
		t.Fatal("expected a node, got nil")
	}
	if n.Type != TypeNull {
		t.Fatalf("expected node type to be initialized to TypeNull, got %v", n.Type)
	}

	// Manipulate
	n.Type = TypeObject
	n.Child = Get()
	n.Child.Type = TypeString

	// Put back
	Put(n)
}

func BenchmarkNodeAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := Get()
		n.Type = TypeObject
		child := Get()
		child.Type = TypeString
		n.Child = child

		Put(n)
	}
}
