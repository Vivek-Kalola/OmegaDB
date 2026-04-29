<div align="center">

# ⚡ OmegaDB

**A high-performance, schema-driven JSON document database built in Go.**

Zero-allocation parsing · Strict schema enforcement · O(1) indexed queries · WAL-backed persistence

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

</div>

---

## Why OmegaDB?

Most Go JSON databases force you to choose between **speed** and **safety**. OmegaDB gives you both:

| Problem with existing tools | OmegaDB's answer |
|---|---|
| `encoding/json` unmarshals to `interface{}` — 20 allocations per parse | Lazy AST: values stay as `[]byte` until you ask for them — **0 allocations** |
| No schema enforcement at write time | Strict schema validation rejects bad documents at `Insert` |
| Full document scan for every query | Hash Indexes deliver **O(1)** exact-match lookups |
| In-memory only — data lost on restart | **WAL + Snapshot** persistence — data survives crashes |
| Reflection-heavy libraries | Zero-reflection, zero-copy tokenizer |

---

## Feature Overview

- **Lazy AST parsing** — JSON is tokenized into a node tree; strings and numbers remain as raw `[]byte` slices until accessed
- **Strict schema validation** — define expected fields, types, and required constraints; violations are rejected at insertion
- **Hash indexing** — create secondary indexes on any JSON path in one line; automatically maintained on all writes
- **Dot-notation query engine** — compiled OpCode plans with `sync.Map` caching; supports `[n]` array index, `[*]` wildcard, `...` recursive DFS
- **Full CRUD** — `Insert`, `Get`, `Update`, `Delete`, `Find`, `Extract`
- **WAL + Snapshot persistence** — every write is fsynced to disk before the in-memory state changes; crash recovery is automatic
- **Zero-copy internals** — `unsafe.Pointer` byte↔string conversion eliminates the last allocation hotspot in the index path
- **Concurrency-safe** — `sync.RWMutex` throughout; passes Go's race detector

---

## Quick Start

### In-Memory Collection (no persistence)

```go
import "OmegaDB/db"

col := db.NewCollection(nil) // nil = no schema, accept any JSON

// Insert — returns a UUID
id, err := col.Insert([]byte(`{"email": "dev@omega.db", "role": "admin"}`))

// Get by ID
doc, err := col.Get(id)
fmt.Println(string(doc.Raw))

// Update (atomic replace, indexes updated automatically)
col.Update(id, []byte(`{"email": "dev@omega.db", "role": "superadmin"}`))

// Delete
col.Delete(id)
```

### Persistent Collection (WAL + Snapshot)

```go
// First run — creates ./mydb/collection.wal and .snap
col, err := db.OpenCollection("./mydb", nil)
if err != nil { log.Fatal(err) }

col.Insert([]byte(`{"user": "alice", "city": "Mumbai"}`))

col.Close() // flushes snapshot, truncates WAL

// Next run — data restored automatically from snapshot + WAL replay
col, _ = db.OpenCollection("./mydb", nil)
defer col.Close()
```

### Schema-Enforced Collection

```go
import (
    "OmegaDB/db"
    "OmegaDB/schema"
)

s := &schema.Schema{
    Root: &schema.Field{
        Type: schema.TypeObject,
        Properties: map[string]*schema.Field{
            "email":  {Type: schema.TypeString, Required: true},
            "active": {Type: schema.TypeBool,   Required: true},
            "score":  {Type: schema.TypeNumber,  Required: false},
        },
    },
}

col := db.NewCollection(s)

// ✅ Valid
col.Insert([]byte(`{"email": "a@b.com", "active": true}`))

// ❌ Rejected — missing required "email"
col.Insert([]byte(`{"active": true}`))

// ❌ Rejected — unexpected field not in schema
col.Insert([]byte(`{"email": "a@b.com", "active": true, "extra": 1}`))
```

---

## Hash Indexing

```go
col := db.NewCollection(nil)

// Create a hash index on any JSON path
col.CreateIndex("user.email")

// Insert some documents
col.Insert([]byte(`{"user": {"email": "alice@test.com", "plan": "pro"}}`))
col.Insert([]byte(`{"user": {"email": "bob@test.com",   "plan": "free"}}`))
col.Insert([]byte(`{"user": {"email": "alice@test.com", "plan": "pro"}}`))

// O(1) lookup — returns all matching document IDs
ids, _ := col.FindByExactMatch("user.email", []byte(`"alice@test.com"`))
// ids → ["<uuid1>", "<uuid2>"]

// Smart Find — uses index if available, table-scan fallback otherwise
ids, _ = col.Find("user.email", []byte(`"bob@test.com"`))
```

---

## Query Engine

```go
// Dot-notation: extract a nested field
vals, _ := col.Extract(id, "user.profile.name")
// → [[]byte(`"Alice"`)]

// Array index: get the second element
vals, _ = col.Extract(id, "orders[1].total")
// → [[]byte(`299.99`)]

// Wildcard: get all tags
vals, _ = col.Extract(id, "tags[*]")
// → [[]byte(`"go"`), []byte(`"database"`), []byte(`"fast"`)]

// Recursive DFS: find "name" anywhere in the document
vals, _ = col.Extract(id, "...name")
// → all matching nodes, depth-first
```

---

## Benchmarks

Measured on **Apple M4 Pro**, Go 1.25, `-bench=. -benchmem`.

### Parser vs `encoding/json`

```
goos: darwin / goarch: arm64 / cpu: Apple M4 Pro

BenchmarkParse-14             7,098,631    147.4 ns/op    0 B/op    0 allocs/op
BenchmarkEncodingJSON-14      2,138,062    559.8 ns/op  688 B/op   20 allocs/op
```

> OmegaDB is **3.8× faster** with **zero allocations** vs the standard library.

### Zero-Copy String Conversion

```
BenchmarkBytesToStringZeroCopy-14    1,000,000,000    0.237 ns/op    0 B/op    0 allocs/op
BenchmarkBytesToStringStdlib-14        729,823,424    1.646 ns/op    0 B/op    0 allocs/op
```

> The `unsafe.Pointer` zero-copy path is **7× faster** — used in every index lookup.

### Node Pool

```
BenchmarkNodeAlloc-14    69,914,371    17.26 ns/op    0 B/op    0 allocs/op
```

> `sync.Pool` recycling keeps the GC completely out of the hot path.

---

## Use Cases

### 1. Configuration & Feature Flag Store
Store structured config JSON with strict schemas. Look up feature flags by key in O(1) without unmarshal overhead.

### 2. Event / Audit Log
Insert append-only event documents durably via WAL. Use `Extract("...timestamp")` to pull timestamps across any event shape.

### 3. Session Store
Fast in-memory collection for HTTP session data. Hash-index on `session_id` for microsecond lookups. Optionally persist to disk for restart resilience.

### 4. Embedded Database for CLI Tools
Ship OmegaDB inside a Go CLI to store user preferences, cached responses, or local state without pulling in a heavy database dependency.

### 5. IoT / Edge Data Collector
Schema-enforce incoming sensor JSON payloads at ingest. Index on `device_id` or `sensor_type` for real-time per-device queries.

### 6. Test Data Fixture Store
Load test fixtures as JSON, query them via dot-notation paths in unit tests without writing custom fixture loaders.

---

## Project Structure

```
OmegaDB/
├── node/        Zero-allocation lazy AST (NodeType, Node, sync.Pool)
├── parser/      Non-reflective JSON tokenizer (no encoding/json, no reflect)
├── schema/      Strict schema definition and validation
├── db/          Core API — Collection, Document, Index
│   ├── collection.go   Insert, Get, Update, Delete, Find, Extract
│   ├── document.go     Document wrapper + node pool lifecycle
│   └── index.go        Hash index engine
├── query/       OpCode compiler + DFS executor
│   ├── plan.go         Compile() — dot-notation → []Op, cached in sync.Map
│   └── execute.go      Execute() — AST traversal with short-circuit
├── wal/         Write-Ahead Log (binary append, fsync, crash-tolerant replay)
├── store/       Snapshot engine (atomic write-rename, binary format)
└── internal/    unsafe.Pointer zero-copy byte↔string helpers
```

---

## Roadmap

- [ ] B-Tree index for range queries (`age > 25`)
- [ ] TTL support for document expiry
- [ ] Multi-collection `Database` wrapper
- [ ] HTTP/gRPC server mode
- [ ] SIMD-accelerated scanning (Phase 4 stretch goal)
- [ ] Compaction — merge WAL + snapshot during idle time

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
