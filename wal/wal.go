// Package wal implements a Write-Ahead Log for OmegaDB.
//
// Every mutation (Insert, Update, Delete) is durably appended to the WAL file
// before the in-memory state is modified. On restart, the WAL is replayed to
// recover any writes that occurred after the last snapshot.
//
// Binary entry format (little-endian):
//
//	[1 byte op] [4 byte id_len] [id bytes] [4 byte data_len] [data bytes]
//
// Op codes:
//
//	'I' = Insert
//	'D' = Delete
//	'U' = Update (replace data for existing ID)
package wal

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"
)

const (
	OpInsert byte = 'I'
	OpDelete byte = 'D'
	OpUpdate byte = 'U'
)

// WALEntry is a single mutation record.
type WALEntry struct {
	Op   byte
	ID   string
	Data []byte // nil for Delete
}

// WAL is a write-ahead log backed by an append-only file.
type WAL struct {
	mu   sync.Mutex
	file *os.File
	buf  *bufio.Writer
}

// Open opens (or creates) the WAL file at path in append mode.
func Open(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{
		file: f,
		buf:  bufio.NewWriterSize(f, 64*1024), // 64 KB write buffer
	}, nil
}

// Append durably writes a WALEntry to the log and syncs to disk.
func (w *WAL) Append(entry WALEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	idBytes := []byte(entry.ID)

	// [1 byte op]
	if err := w.buf.WriteByte(entry.Op); err != nil {
		return err
	}

	// [4 byte id_len]
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(idBytes)))
	if _, err := w.buf.Write(lenBuf[:]); err != nil {
		return err
	}

	// [id bytes]
	if _, err := w.buf.Write(idBytes); err != nil {
		return err
	}

	// [4 byte data_len]
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(entry.Data)))
	if _, err := w.buf.Write(lenBuf[:]); err != nil {
		return err
	}

	// [data bytes]
	if len(entry.Data) > 0 {
		if _, err := w.buf.Write(entry.Data); err != nil {
			return err
		}
	}

	// Flush buffer → OS and fsync → disk for durability
	if err := w.buf.Flush(); err != nil {
		return err
	}
	return w.file.Sync()
}

// Replay reads all WAL entries from the beginning of the file and calls fn for each.
// It tolerates a truncated final entry (crash mid-write) by stopping early.
func (w *WAL) Replay(fn func(WALEntry)) error {
	// Seek to start for reading
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	r := bufio.NewReader(w.file)
	var lenBuf [4]byte

	for {
		// Read op byte
		op, err := r.ReadByte()
		if errors.Is(err, io.EOF) {
			break // clean end of file
		}
		if err != nil {
			return err
		}

		// Read id_len
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			break // truncated — stop, partial writes are ignored
		}
		idLen := binary.LittleEndian.Uint32(lenBuf[:])

		// Read id
		idBytes := make([]byte, idLen)
		if _, err := io.ReadFull(r, idBytes); err != nil {
			break
		}

		// Read data_len
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			break
		}
		dataLen := binary.LittleEndian.Uint32(lenBuf[:])

		// Read data
		var data []byte
		if dataLen > 0 {
			data = make([]byte, dataLen)
			if _, err := io.ReadFull(r, data); err != nil {
				break
			}
		}

		fn(WALEntry{Op: op, ID: string(idBytes), Data: data})
	}

	return nil
}

// Truncate discards all WAL entries (called after a successful snapshot).
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.buf.Flush(); err != nil {
		return err
	}
	if err := w.file.Truncate(0); err != nil {
		return err
	}
	_, err := w.file.Seek(0, io.SeekStart)
	w.buf.Reset(w.file)
	return err
}

// Close flushes and closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.buf.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}
