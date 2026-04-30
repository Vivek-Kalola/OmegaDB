package schema

import (
	"github.com/Vivek-Kalola/omega-db/node"
	"testing"
)

func TestSchemaValidation(t *testing.T) {
	s := &Schema{
		Root: &Field{
			Type: TypeObject,
			Properties: map[string]*Field{
				"name": {Type: TypeString, Required: true},
				"age":  {Type: TypeNumber, Required: false},
				"tags": {
					Type:  TypeArray,
					Items: &Field{Type: TypeString},
				},
			},
		},
	}

	// Valid node
	root := node.Get()
	root.Type = node.TypeObject

	nameNode := node.Get()
	nameNode.Type = node.TypeString
	nameNode.Key = []byte("name")

	ageNode := node.Get()
	ageNode.Type = node.TypeNumber
	ageNode.Key = []byte("age")

	tagsNode := node.Get()
	tagsNode.Type = node.TypeArray
	tagsNode.Key = []byte("tags")

	tag1 := node.Get()
	tag1.Type = node.TypeString

	tagsNode.Child = tag1

	root.Child = nameNode
	nameNode.Next = ageNode
	ageNode.Next = tagsNode

	err := s.Validate(root)
	if err != nil {
		t.Fatalf("expected validation to pass, got error: %v", err)
	}

	// Invalid node (missing required field)
	root2 := node.Get()
	root2.Type = node.TypeObject
	err = s.Validate(root2)
	if err == nil {
		t.Fatal("expected validation to fail due to missing 'name'")
	}

	node.Put(root)
	node.Put(root2)
}
