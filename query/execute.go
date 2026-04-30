package query

import (
	"bytes"

	"github.com/Vivek-Kalola/omega-db/node"
)

// Execute runs the compiled query plan against a root node and returns all matching raw JSON slices.
func Execute(plan *Plan, root *node.Node) [][]byte {
	var results [][]byte
	executeOps(plan.Ops, root, &results)
	return results
}

// executeOps recursively processes the OpCodes against the AST.
func executeOps(ops []Op, curr *node.Node, results *[][]byte) {
	if curr == nil {
		return
	}

	// Base case: no more operations, we matched the target node
	if len(ops) == 0 {
		*results = append(*results, curr.Raw)
		return
	}

	op := ops[0]
	nextOps := ops[1:]

	switch op.Type {
	case OpField:
		if curr.Type != node.TypeObject {
			return
		}
		child := curr.Child
		for child != nil {
			if bytes.Equal(child.Key, op.Field) {
				executeOps(nextOps, child, results)
				// Assuming standard JSON without duplicate keys
				break
			}
			child = child.Next
		}

	case OpIndex:
		if curr.Type != node.TypeArray {
			return
		}
		child := curr.Child
		i := 0
		for child != nil {
			if i == op.Index {
				executeOps(nextOps, child, results)
				break
			}
			child = child.Next
			i++
		}

	case OpWildcard:
		if curr.Type == node.TypeObject || curr.Type == node.TypeArray {
			child := curr.Child
			for child != nil {
				executeOps(nextOps, child, results)
				child = child.Next
			}
		}

	case OpRecursive:
		// 1. Try to match the NEXT operations against the current node
		executeOps(nextOps, curr, results)

		// 2. Continue the recursive descent down the AST
		if curr.Type == node.TypeObject || curr.Type == node.TypeArray {
			child := curr.Child
			for child != nil {
				// Pass the original ops (including OpRecursive) to children
				executeOps(ops, child, results)
				child = child.Next
			}
		}
	}
}
