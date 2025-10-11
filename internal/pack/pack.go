package pack

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// Git pack header constants.
var (
	magicPACK          = []byte{'P', 'A', 'C', 'K'}
	packVersion uint32 = 2 // widely supported
)

// Git object type codes (header nibble). We emit blobs here.
const (
	_ = iota + 1
	_
	objBlob //nolint:unused // Used in Object.Type field and writeObjHeader
	_
	// 6 & 7 are reserved for OFS_DELTA/REF_DELTA (not used in this minimal writer).
)

type CompressAlgo int

const (
	CompressZlib CompressAlgo = iota
	CompressZstd
)

type Object struct {
	Type int          // objBlob / objTree / objCommit / objTag (we’ll use objBlob here)
	Size uint64       // uncompressed size
	Data []byte       // uncompressed canonical bytes ("blob <len>\x00"+"content" for blobs)
	Algo CompressAlgo // per-object compression selection
}

// writeObjHeader writes the varint header: low bits = size; type in bits 4..6; MSB is continuation.
func writeObjHeader(w io.Writer, objType int, size uint64) error {
	if objType < 0 || objType > 7 {
		return fmt.Errorf("invalid objType %d", objType)
	}
	// First byte carries type and 4 LSB of size.
	b := byte((objType&7)<<4) | byte(size&0x0F)
	size >>= 4
	if size != 0 {
		b |= 0x80 // continuation
	}
	if _, err := w.Write([]byte{b}); err != nil {
		return err
	}
	// Subsequent bytes carry remaining size 7 bits at a time with MSB continuation.
	for size != 0 {
		c := byte(size & 0x7F)
		size >>= 7
		if size != 0 {
			c |= 0x80
		}
		if _, err := w.Write([]byte{c}); err != nil {
			return err
		}
	}
	return nil
}

func compressZlib(dst *bytes.Buffer, data []byte) error {
	zw := zlib.NewWriter(dst)
	if _, err := zw.Write(data); err != nil {
		return err
	}
	return zw.Close()
}

func compressZstd(dst *bytes.Buffer, data []byte) error {
	zw, err := zstd.NewWriter(dst, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return err
	}
	if _, err := zw.Write(data); err != nil {
		return err
	}
	return zw.Close()
}

// WritePack writes a minimal packfile containing the provided objects.
// If trailerSHA256 is true, appends a SHA-256 of the entire pack stream
// (useful for SHA-256 repos; traditional SHA-1 trailer not included here).
func WritePack(objs []Object, trailerSHA256 bool) ([]byte, error) {
	var body bytes.Buffer

	// Header: "PACK" + version + count
	if _, err := body.Write(magicPACK); err != nil {
		return nil, err
	}
	if err := binary.Write(&body, binary.BigEndian, packVersion); err != nil {
		return nil, err
	}
	count := uint32(len(objs))
	if err := binary.Write(&body, binary.BigEndian, count); err != nil {
		return nil, err
	}

	// Objects
	for _, o := range objs {
		if err := writeObjHeader(&body, o.Type, o.Size); err != nil {
			return nil, fmt.Errorf("write header: %w", err)
		}
		var cbuf bytes.Buffer
		switch o.Algo {
		case CompressZlib:
			if err := compressZlib(&cbuf, o.Data); err != nil {
				return nil, err
			}
		case CompressZstd:
			// NOTE: zstd in packfiles requires modern Git; fallback to zlib if remote lacks support.
			if err := compressZstd(&cbuf, o.Data); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown compression algo")
		}
		if _, err := body.Write(cbuf.Bytes()); err != nil {
			return nil, err
		}
	}

	// Optional SHA-256 trailer (non-legacy; for SHA-256 repos).
	if trailerSHA256 {
		sum := sha256.Sum256(body.Bytes())
		if _, err := body.Write(sum[:]); err != nil {
			return nil, err
		}
	}

	return body.Bytes(), nil
}
