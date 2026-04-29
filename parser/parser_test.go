package parser

import (
	"OmegaDB/node"
	"encoding/json"
	"testing"
)

func TestParseValidJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty object", []byte(`{}`)},
		{"simple object", []byte(`{"key": "value"}`)},
		{"nested object", []byte(`{"a": {"b": 1}}`)},
		{"array", []byte(`[1, 2, 3]`)},
		{"mixed", []byte(`{"a": [1, true, null, "string"], "b": false}`)},
		{"complex", []byte(`
			{
				"id": "123",
				"active": true,
				"balance": 100.50,
				"tags": ["a", "b"],
				"meta": null
			}
		`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := Parse(tt.data)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if n == nil {
				t.Fatal("expected node, got nil")
			}
			node.Put(n)
		})
	}
}

func BenchmarkParse(b *testing.B) {
	data := []byte(`{"id":"123","active":true,"balance":100.50,"tags":["a","b"],"meta":null}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n, err := Parse(data)
		if err != nil {
			b.Fatal(err)
		}
		node.Put(n)
	}
}

func BenchmarkEncodingJSON(b *testing.B) {
	data := []byte(`{"id":"123","active":true,"balance":100.50,"tags":["a","b"],"meta":null}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			b.Fatal(err)
		}
	}
}
