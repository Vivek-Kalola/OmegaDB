package query

import (
	"testing"
)

func TestCompileDotNotation(t *testing.T) {
	plan, err := Compile("user.profile.id")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if len(plan.Ops) != 3 {
		t.Fatalf("Expected 3 ops, got %d", len(plan.Ops))
	}
	assertField(t, plan.Ops[0], "user")
	assertField(t, plan.Ops[1], "profile")
	assertField(t, plan.Ops[2], "id")
}

func TestCompileArrayIndex(t *testing.T) {
	plan, err := Compile("items[0].price")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if len(plan.Ops) != 3 {
		t.Fatalf("Expected 3 ops, got %d", len(plan.Ops))
	}
	assertField(t, plan.Ops[0], "items")
	assertIndex(t, plan.Ops[1], 0)
	assertField(t, plan.Ops[2], "price")
}

func TestCompileWildcard(t *testing.T) {
	plan, err := Compile("tags[*]")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if len(plan.Ops) != 2 {
		t.Fatalf("Expected 2 ops, got %d", len(plan.Ops))
	}
	assertField(t, plan.Ops[0], "tags")
	assertOp(t, plan.Ops[1], OpWildcard)
}

func TestCompileRecursiveDescent(t *testing.T) {
	plan, err := Compile("...name")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if len(plan.Ops) != 2 {
		t.Fatalf("Expected 2 ops, got %d", len(plan.Ops))
	}
	assertOp(t, plan.Ops[0], OpRecursive)
	assertField(t, plan.Ops[1], "name")
}

func TestPlanCacheHit(t *testing.T) {
	// Compile twice – second call must return from cache
	p1, _ := Compile("a.b.c")
	p2, _ := Compile("a.b.c")
	if p1 != p2 {
		t.Fatal("Expected same plan pointer from cache")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func assertField(t *testing.T, op Op, name string) {
	t.Helper()
	if op.Type != OpField {
		t.Fatalf("Expected OpField, got %v", op.Type)
	}
	if string(op.Field) != name {
		t.Fatalf("Expected field '%s', got '%s'", name, op.Field)
	}
}

func assertIndex(t *testing.T, op Op, idx int) {
	t.Helper()
	if op.Type != OpIndex {
		t.Fatalf("Expected OpIndex, got %v", op.Type)
	}
	if op.Index != idx {
		t.Fatalf("Expected index %d, got %d", idx, op.Index)
	}
}

func assertOp(t *testing.T, op Op, expected OpType) {
	t.Helper()
	if op.Type != expected {
		t.Fatalf("Expected OpType %v, got %v", expected, op.Type)
	}
}
