// Package internal provides low-level, performance-critical utilities used
// across omega-db packages. Functions here use unsafe but are safe in the
// specific context of omega-db because underlying byte slices are never mutated
// after creation.
package internal

import "unsafe"

// BytesToString converts a byte slice to a string with zero allocations.
// The returned string shares memory with b — do NOT mutate b after calling this.
func BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytes converts a string to a byte slice with zero allocations.
// The returned slice shares memory with s — do NOT mutate the slice.
func StringToBytes(s string) []byte {
	// Represent a string header as a slice header with the same pointer.
	// The Cap field is set to Len to produce a valid slice header.
	type stringHeader struct {
		Data uintptr
		Len  int
	}
	type sliceHeader struct {
		Data uintptr
		Len  int
		Cap  int
	}
	sh := (*stringHeader)(unsafe.Pointer(&s))
	bh := sliceHeader{Data: sh.Data, Len: sh.Len, Cap: sh.Len}
	return *(*[]byte)(unsafe.Pointer(&bh))
}
