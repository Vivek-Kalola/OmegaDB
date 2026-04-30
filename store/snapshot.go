// Package store implements snapshot serialization for omega-db.
//
// A snapshot is a point-in-time binary dump of all documents in a collection.
// It is used at startup to restore in-memory state quickly, after which the WAL
// is replayed to catch up any writes that occurred after the snapshot was taken.
//
// Snapshot file format:
//
//	For each document:
//	  [4 byte id_len] [id bytes] [4 byte data_len] [data bytes]
//	Terminated by a zero id_len (0x00000000) sentinel.
package store

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// SnapshotDoc is a lightweight record used for snapshot I/O.
type SnapshotDoc struct {
	ID   string
	Data []byte
}

// WriteSnapshot atomically writes all documents to path.
// It writes to a temporary file first then renames to ensure atomicity.
func WriteSnapshot(path string, docs []SnapshotDoc) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	w := bufio.NewWriterSize(f, 256*1024) // 256 KB write buffer
	var lenBuf [4]byte

	for _, doc := range docs {
		idBytes := []byte(doc.ID)

		// id_len + id
		binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(idBytes)))
		if _, err := w.Write(lenBuf[:]); err != nil {
			f.Close()
			return err
		}
		if _, err := w.Write(idBytes); err != nil {
			f.Close()
			return err
		}

		// data_len + data
		binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(doc.Data)))
		if _, err := w.Write(lenBuf[:]); err != nil {
			f.Close()
			return err
		}
		if _, err := w.Write(doc.Data); err != nil {
			f.Close()
			return err
		}
	}

	// Write sentinel: id_len = 0
	binary.LittleEndian.PutUint32(lenBuf[:], 0)
	if _, err := w.Write(lenBuf[:]); err != nil {
		f.Close()
		return err
	}

	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// Atomic rename
	return os.Rename(tmpPath, path)
}

// ReadSnapshot reads all documents from a snapshot file.
// Returns an empty slice (not an error) if the file does not exist.
func ReadSnapshot(path string) ([]SnapshotDoc, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	var lenBuf [4]byte
	var docs []SnapshotDoc

	for {
		// id_len
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, err
		}
		idLen := binary.LittleEndian.Uint32(lenBuf[:])
		if idLen == 0 {
			break // sentinel reached
		}

		// id
		idBytes := make([]byte, idLen)
		if _, err := io.ReadFull(r, idBytes); err != nil {
			return nil, err
		}

		// data_len
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			return nil, err
		}
		dataLen := binary.LittleEndian.Uint32(lenBuf[:])

		// data
		data := make([]byte, dataLen)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}

		docs = append(docs, SnapshotDoc{ID: string(idBytes), Data: data})
	}

	return docs, nil
}
