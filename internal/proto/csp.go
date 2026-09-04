package proto

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// CRLF terminates CSP headers and bulk payloads.
const CRLF = "\r\n"

var (
	// ErrProtocol is returned when the buffer holds an invalid CSP value.
	ErrProtocol = errors.New("proto: protocol error")
)

// Kind identifies a CSP value type.
type Kind int

const (
	KindSimple Kind = iota
	KindError
	KindInteger
	KindBulk
	KindNull // null bulk: $-1\r\n
	KindArray
)

// Value is one decoded or to-be-encoded CSP value.
type Value struct {
	Kind Kind
	Str  string  // Simple, Error, Bulk
	Int  int64   // Integer
	Arr  []Value // Array
}

// Simple builds a simple-string value (+OK / +PONG).
func Simple(s string) Value { return Value{Kind: KindSimple, Str: s} }

// ErrorMsg builds an error value (-ERR ...).
func ErrorMsg(msg string) Value { return Value{Kind: KindError, Str: msg} }

// Errorf builds a formatted error value.
func Errorf(format string, args ...any) Value {
	return ErrorMsg(fmt.Sprintf(format, args...))
}

// Integer builds an integer value.
func Integer(n int64) Value { return Value{Kind: KindInteger, Int: n} }

// Bulk builds a bulk-string value.
func Bulk(s string) Value { return Value{Kind: KindBulk, Str: s} }

// Null builds a null bulk (GET miss).
func Null() Value { return Value{Kind: KindNull} }

// Array builds an array value.
func Array(items ...Value) Value { return Value{Kind: KindArray, Arr: items} }

// Encode appends the CSP encoding of v to dst and returns the extended slice.
func Encode(dst []byte, v Value) []byte {
	switch v.Kind {
	case KindSimple:
		dst = append(dst, '+')
		dst = append(dst, v.Str...)
		return append(dst, CRLF...)
	case KindError:
		dst = append(dst, '-')
		dst = append(dst, v.Str...)
		return append(dst, CRLF...)
	case KindInteger:
		dst = append(dst, ':')
		dst = strconv.AppendInt(dst, v.Int, 10)
		return append(dst, CRLF...)
	case KindBulk:
		dst = append(dst, '$')
		dst = strconv.AppendInt(dst, int64(len(v.Str)), 10)
		dst = append(dst, CRLF...)
		dst = append(dst, v.Str...)
		return append(dst, CRLF...)
	case KindNull:
		return append(dst, '$', '-', '1', '\r', '\n')
	case KindArray:
		dst = append(dst, '*')
		dst = strconv.AppendInt(dst, int64(len(v.Arr)), 10)
		dst = append(dst, CRLF...)
		for _, item := range v.Arr {
			dst = Encode(dst, item)
		}
		return dst
	default:
		return Encode(dst, ErrorMsg("unknown value kind"))
	}
}

// EncodeValue returns a new buffer with v encoded.
func EncodeValue(v Value) []byte {
	return Encode(nil, v)
}

// Decode reads one complete CSP value from buf.
//
// On success ok is true, v is the value, and rest is the unread suffix.
// If buf is incomplete, ok is false, err is nil, and rest is buf (caller waits).
// If buf is corrupt, err is non-nil.
func Decode(buf []byte) (v Value, rest []byte, ok bool, err error) {
	if len(buf) == 0 {
		return Value{}, buf, false, nil
	}
	return decodeValue(buf)
}

func decodeValue(buf []byte) (Value, []byte, bool, error) {
	if len(buf) == 0 {
		return Value{}, buf, false, nil
	}
	switch buf[0] {
	case '+':
		line, rest, complete, err := readLine(buf[1:])
		if err != nil || !complete {
			return Value{}, buf, false, err
		}
		return Value{Kind: KindSimple, Str: string(line)}, rest, true, nil
	case '-':
		line, rest, complete, err := readLine(buf[1:])
		if err != nil || !complete {
			return Value{}, buf, false, err
		}
		return Value{Kind: KindError, Str: string(line)}, rest, true, nil
	case ':':
		line, rest, complete, err := readLine(buf[1:])
		if err != nil || !complete {
			return Value{}, buf, false, err
		}
		n, perr := strconv.ParseInt(string(line), 10, 64)
		if perr != nil {
			return Value{}, buf, false, fmt.Errorf("%w: bad integer %q", ErrProtocol, line)
		}
		return Value{Kind: KindInteger, Int: n}, rest, true, nil
	case '$':
		line, rest, complete, err := readLine(buf[1:])
		if err != nil || !complete {
			return Value{}, buf, false, err
		}
		n, perr := strconv.Atoi(string(line))
		if perr != nil {
			return Value{}, buf, false, fmt.Errorf("%w: bad bulk length %q", ErrProtocol, line)
		}
		if n == -1 {
			return Value{Kind: KindNull}, rest, true, nil
		}
		if n < 0 {
			return Value{}, buf, false, fmt.Errorf("%w: negative bulk length", ErrProtocol)
		}
		need := n + 2 // payload + CRLF
		if len(rest) < need {
			return Value{}, buf, false, nil
		}
		payload := rest[:n]
		if rest[n] != '\r' || rest[n+1] != '\n' {
			return Value{}, buf, false, fmt.Errorf("%w: bulk missing CRLF", ErrProtocol)
		}
		return Value{Kind: KindBulk, Str: string(payload)}, rest[need:], true, nil
	case '*':
		line, rest, complete, err := readLine(buf[1:])
		if err != nil || !complete {
			return Value{}, buf, false, err
		}
		n, perr := strconv.Atoi(string(line))
		if perr != nil || n < 0 {
			return Value{}, buf, false, fmt.Errorf("%w: bad array length %q", ErrProtocol, line)
		}
		items := make([]Value, 0, n)
		cur := rest
		for i := 0; i < n; i++ {
			item, next, itemOK, itemErr := decodeValue(cur)
			if itemErr != nil {
				return Value{}, buf, false, itemErr
			}
			if !itemOK {
				return Value{}, buf, false, nil
			}
			items = append(items, item)
			cur = next
		}
		return Value{Kind: KindArray, Arr: items}, cur, true, nil
	default:
		return Value{}, buf, false, fmt.Errorf("%w: unknown type byte %q", ErrProtocol, buf[0])
	}
}

// readLine splits the first CRLF-terminated line from buf (without the CRLF).
// complete=false if no complete line yet. Lone LF without CR is a protocol error.
func readLine(buf []byte) (line, rest []byte, complete bool, err error) {
	i := bytes.IndexByte(buf, '\n')
	if i < 0 {
		return nil, buf, false, nil
	}
	if i == 0 || buf[i-1] != '\r' {
		return nil, buf, false, fmt.Errorf("%w: expected CRLF", ErrProtocol)
	}
	return buf[:i-1], buf[i+1:], true, nil
}

// CommandArgs extracts verb+args from a client Array-of-bulks request.
func CommandArgs(v Value) (args []string, err error) {
	if v.Kind != KindArray {
		return nil, fmt.Errorf("%w: command must be an array", ErrProtocol)
	}
	if len(v.Arr) == 0 {
		return nil, fmt.Errorf("%w: empty command array", ErrProtocol)
	}
	args = make([]string, len(v.Arr))
	for i, item := range v.Arr {
		switch item.Kind {
		case KindBulk, KindSimple:
			args[i] = item.Str
		case KindInteger:
			args[i] = strconv.FormatInt(item.Int, 10)
		case KindNull:
			args[i] = ""
		default:
			return nil, fmt.Errorf("%w: command arg %d has unsupported type", ErrProtocol, i)
		}
	}
	return args, nil
}
