package internal

import (
	"testing"
)

func TestBytesToString(t *testing.T) {
	b := []byte("hello-omega")
	s := BytesToString(b)
	if s != "hello-omega" {
		t.Fatalf("expected 'hello-omega', got '%s'", s)
	}
}

func TestStringToBytes(t *testing.T) {
	s := "hello-omega"
	b := StringToBytes(s)
	if string(b) != s {
		t.Fatalf("expected '%s', got '%s'", s, b)
	}
}

func BenchmarkBytesToStringZeroCopy(b *testing.B) {
	data := []byte("some-index-key-value")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = BytesToString(data)
	}
}

func BenchmarkBytesToStringStdlib(b *testing.B) {
	data := []byte("some-index-key-value")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = string(data)
	}
}
