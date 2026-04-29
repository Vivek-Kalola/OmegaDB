package query

import (
	"bytes"
	"testing"

	"OmegaDB/node"
)

// buildTree constructs a small in-memory AST for test purposes.
//
// Represents:
//
//	{
//	  "user": {
//	    "name": "Alice",
//	    "scores": [10, 20, 30]
//	  },
//	  "active": true
//	}
func buildTree() *node.Node {
	// scores array: [10, 20, 30]
	s1 := node.Get()
	s1.Type = node.TypeNumber
	s1.Raw = []byte("10")
	s2 := node.Get()
	s2.Type = node.TypeNumber
	s2.Raw = []byte("20")
	s3 := node.Get()
	s3.Type = node.TypeNumber
	s3.Raw = []byte("30")
	s1.Next = s2
	s2.Next = s3

	scores := node.Get()
	scores.Type = node.TypeArray
	scores.Key = []byte("scores")
	scores.Child = s1

	name := node.Get()
	name.Type = node.TypeString
	name.Key = []byte("name")
	name.Raw = []byte(`"Alice"`)
	name.Next = scores

	user := node.Get()
	user.Type = node.TypeObject
	user.Key = []byte("user")
	user.Child = name

	active := node.Get()
	active.Type = node.TypeBool
	active.Key = []byte("active")
	active.Raw = []byte("true")

	user.Next = active

	root := node.Get()
	root.Type = node.TypeObject
	root.Child = user

	return root
}

func TestExecuteField(t *testing.T) {
	root := buildTree()
	defer node.Put(root)

	plan, _ := Compile("user.name")
	results := Execute(plan, root)
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if !bytes.Equal(results[0], []byte(`"Alice"`)) {
		t.Fatalf("Expected '\"Alice\"', got '%s'", results[0])
	}
}

func TestExecuteArrayIndex(t *testing.T) {
	root := buildTree()
	defer node.Put(root)

	plan, _ := Compile("user.scores[1]")
	results := Execute(plan, root)
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if !bytes.Equal(results[0], []byte("20")) {
		t.Fatalf("Expected '20', got '%s'", results[0])
	}
}

func TestExecuteWildcard(t *testing.T) {
	root := buildTree()
	defer node.Put(root)

	plan, _ := Compile("user.scores[*]")
	results := Execute(plan, root)
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
}

func TestExecuteRecursiveDFS(t *testing.T) {
	root := buildTree()
	defer node.Put(root)

	plan, _ := Compile("...name")
	results := Execute(plan, root)
	if len(results) != 1 {
		t.Fatalf("Expected 1 result from DFS, got %d", len(results))
	}
	if !bytes.Equal(results[0], []byte(`"Alice"`)) {
		t.Fatalf("Expected '\"Alice\"', got '%s'", results[0])
	}
}

func TestExecuteMissingPath(t *testing.T) {
	root := buildTree()
	defer node.Put(root)

	plan, _ := Compile("user.nonexistent")
	results := Execute(plan, root)
	if len(results) != 0 {
		t.Fatalf("Expected 0 results for missing path, got %d", len(results))
	}
}
