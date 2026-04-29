package schema

import (
	"OmegaDB/node"
	"fmt"
)

// FieldType defines the expected type of a JSON field in the schema.
type FieldType string

const (
	TypeObject FieldType = "object"
	TypeArray  FieldType = "array"
	TypeString FieldType = "string"
	TypeNumber FieldType = "number"
	TypeBool   FieldType = "boolean"
	TypeNull   FieldType = "null"
)

// Field defines the structure and constraints of a specific JSON key/value.
type Field struct {
	Type     FieldType
	Required bool

	// Properties defines the fields of an object, keyed by the object's keys.
	// Used only when Type == TypeObject.
	Properties map[string]*Field

	// Items defines the schema for elements in an array.
	// Used only when Type == TypeArray.
	Items *Field
}

// Schema represents the expected structure of a JSON document.
type Schema struct {
	Root *Field
}

// Validate ensures that the provided AST Node strictly conforms to the Schema.
func (s *Schema) Validate(n *node.Node) error {
	if s.Root == nil {
		return nil // No schema defined, implicitly valid (though user wanted strict, empty schema means no constraints)
	}
	return validateNode(n, s.Root, "$")
}

func validateNode(n *node.Node, field *Field, path string) error {
	if n == nil {
		return fmt.Errorf("node is nil at path %s", path)
	}

	// 1. Check Type match
	if err := checkTypeMatch(n.Type, field.Type, path); err != nil {
		return err
	}

	// 2. Structural Validation
	switch field.Type {
	case TypeObject:
		return validateObject(n, field, path)
	case TypeArray:
		return validateArray(n, field, path)
	}

	return nil
}

func validateObject(n *node.Node, field *Field, path string) error {
	// Keep track of which required fields we have seen
	seen := make(map[string]bool)

	// Iterate over the object's children
	curr := n.Child
	for curr != nil {
		keyStr := string(curr.Key)

		// Strict schema: if a key is not in Properties, it's an error.
		propField, ok := field.Properties[keyStr]
		if !ok {
			return fmt.Errorf("strict schema violation: unexpected field '%s' at path %s", keyStr, path)
		}

		childPath := path + "." + keyStr
		if err := validateNode(curr, propField, childPath); err != nil {
			return err
		}

		seen[keyStr] = true
		curr = curr.Next
	}

	// Check if all required fields were seen
	for key, propField := range field.Properties {
		if propField.Required && !seen[key] {
			return fmt.Errorf("strict schema violation: missing required field '%s' at path %s", key, path)
		}
	}

	return nil
}

func validateArray(n *node.Node, field *Field, path string) error {
	if field.Items == nil {
		return nil // No schema for items
	}

	curr := n.Child
	index := 0
	for curr != nil {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if err := validateNode(curr, field.Items, itemPath); err != nil {
			return err
		}
		curr = curr.Next
		index++
	}

	return nil
}

func checkTypeMatch(nt node.NodeType, ft FieldType, path string) error {
	switch ft {
	case TypeObject:
		if nt != node.TypeObject {
			return typeMismatchError(ft, nt, path)
		}
	case TypeArray:
		if nt != node.TypeArray {
			return typeMismatchError(ft, nt, path)
		}
	case TypeString:
		if nt != node.TypeString {
			return typeMismatchError(ft, nt, path)
		}
	case TypeNumber:
		if nt != node.TypeNumber {
			return typeMismatchError(ft, nt, path)
		}
	case TypeBool:
		if nt != node.TypeBool {
			return typeMismatchError(ft, nt, path)
		}
	case TypeNull:
		if nt != node.TypeNull {
			return typeMismatchError(ft, nt, path)
		}
	default:
		return fmt.Errorf("unknown schema field type '%s' at path %s", ft, path)
	}
	return nil
}

func typeMismatchError(expected FieldType, actual node.NodeType, path string) error {
	var actualStr string
	switch actual {
	case node.TypeObject:
		actualStr = "object"
	case node.TypeArray:
		actualStr = "array"
	case node.TypeString:
		actualStr = "string"
	case node.TypeNumber:
		actualStr = "number"
	case node.TypeBool:
		actualStr = "boolean"
	case node.TypeNull:
		actualStr = "null"
	default:
		actualStr = "unknown"
	}
	return fmt.Errorf("type mismatch at path %s: expected %s, got %s", path, expected, actualStr)
}
