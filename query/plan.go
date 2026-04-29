package query

import (
	"errors"
	"strconv"
	"strings"
	"sync"
)

// OpType represents the type of operation in a query execution plan.
type OpType int

const (
	OpField     OpType = iota // match exact object key
	OpIndex                   // match array index
	OpWildcard                // match all children (*)
	OpRecursive               // traverse all descendants (...)
)

// Op is a single step in a query execution plan.
type Op struct {
	Type  OpType
	Field []byte // Used for OpField
	Index int    // Used for OpIndex
}

// Plan is a compiled sequence of operations.
type Plan struct {
	Ops []Op
}

// planCache caches compiled plans to avoid re-parsing strings.
var planCache sync.Map

// Compile parses a query string into an executable sequence of OpCodes.
// Supports:
//   - Dot-notation:         user.name
//   - Array index:          items[0]
//   - Wildcard:             tags[*]  or  *
//   - Recursive descent:    ...name  or  ...
func Compile(path string) (*Plan, error) {
	if cached, ok := planCache.Load(path); ok {
		return cached.(*Plan), nil
	}

	var ops []Op
	s := path

	for len(s) > 0 {
		// Consume leading dot separator
		if s[0] == '.' {
			s = s[1:]
			if len(s) == 0 {
				break
			}
		}

		// Recursive descent: starts with ..
		if strings.HasPrefix(s, "..") {
			ops = append(ops, Op{Type: OpRecursive})
			s = s[2:] // consume ".."
			// consume optional trailing dot before next segment
			if len(s) > 0 && s[0] == '.' {
				s = s[1:]
			}
			continue
		}

		// Read next segment (up to '.', '[', or end of string)
		end := strings.IndexAny(s, ".[")
		var segment string
		if end == -1 {
			segment = s
			s = ""
		} else {
			segment = s[:end]
			s = s[end:]
		}

		if segment != "" {
			if segment == "*" {
				ops = append(ops, Op{Type: OpWildcard})
			} else {
				ops = append(ops, Op{Type: OpField, Field: []byte(segment)})
			}
		}

		// Consume any bracket expressions [0], [*], [1][2] etc.
		for len(s) > 0 && s[0] == '[' {
			endIdx := strings.Index(s, "]")
			if endIdx == -1 {
				return nil, errors.New("unclosed bracket in query")
			}
			inside := s[1:endIdx]
			if inside == "*" {
				ops = append(ops, Op{Type: OpWildcard})
			} else {
				i, err := strconv.Atoi(inside)
				if err != nil {
					return nil, errors.New("invalid array index in query")
				}
				ops = append(ops, Op{Type: OpIndex, Index: i})
			}
			s = s[endIdx+1:]
		}
	}

	plan := &Plan{Ops: ops}
	planCache.Store(path, plan)
	return plan, nil
}
