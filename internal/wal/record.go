package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// encodeRecord builds one framed record: len | payload | crc32(payload).
func encodeRecord(e *Entry) ([]byte, error) {
	if e == nil {
		return nil, ErrInvalidEntry
	}
	var opByte byte
	switch e.Op {
	case OpSet:
		opByte = opSetByte
	case OpDelete:
		opByte = opDeleteByte
	default:
		return nil, fmt.Errorf("%w: unknown op %q", ErrInvalidEntry, e.Op)
	}

	key := []byte(e.Key)
	val := []byte(e.Value)
	if e.Op == OpDelete {
		val = nil
	}

	// payload: op | keyLen | key | valLen | val
	payloadLen := 1 + 4 + len(key) + 4 + len(val)
	buf := make([]byte, 4+payloadLen+4)
	binary.BigEndian.PutUint32(buf[0:4], uint32(payloadLen))
	off := 4
	buf[off] = opByte
	off++
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(key)))
	off += 4
	copy(buf[off:], key)
	off += len(key)
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(val)))
	off += 4
	copy(buf[off:], val)
	off += len(val)

	sum := crc32.ChecksumIEEE(buf[4:4+payloadLen])
	binary.BigEndian.PutUint32(buf[off:], sum)
	return buf, nil
}

// decodeRecord parses one payload (without the length prefix / trailing crc).
func decodePayload(payload []byte) (*Entry, error) {
	if len(payload) < 1+4+4 {
		return nil, ErrCorrupt
	}
	opByte := payload[0]
	off := 1
	keyLen := int(binary.BigEndian.Uint32(payload[off : off+4]))
	off += 4
	if keyLen < 0 || off+keyLen+4 > len(payload) {
		return nil, ErrCorrupt
	}
	key := string(payload[off : off+keyLen])
	off += keyLen
	valLen := int(binary.BigEndian.Uint32(payload[off : off+4]))
	off += 4
	if valLen < 0 || off+valLen != len(payload) {
		return nil, ErrCorrupt
	}
	val := string(payload[off : off+valLen])

	var op string
	switch opByte {
	case opSetByte:
		op = OpSet
	case opDeleteByte:
		op = OpDelete
	default:
		return nil, ErrCorrupt
	}
	return &Entry{Op: op, Key: key, Value: val}, nil
}

func encodeHeader() []byte {
	h := make([]byte, headerSize)
	copy(h, magic)
	h[4] = version
	return h
}

func checkHeader(b []byte) error {
	if len(b) < headerSize {
		return ErrBadMagic
	}
	if string(b[0:4]) != magic || b[4] != version {
		return ErrBadMagic
	}
	return nil
}
