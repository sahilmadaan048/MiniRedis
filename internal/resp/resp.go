// Package resp implements the Redis Serialization Protocol (RESP) parser and writer.
package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	prefixString  = '+'
	prefixError   = '-'
	prefixBulk    = '$'
	prefixArray   = '*'
	prefixInteger = ':'
)

// Limits protect the server from clients that announce absurd sizes.
const (
	maxBulkLen  = 512 * 1024 * 1024 // 512 MB, same as Redis
	maxArrayLen = 1024 * 1024
)

// Value is any RESP value. Typ says which of the other fields is meaningful.
type Value struct {
	Typ   string // "array", "bulk", "string", "error", "null"
	Str   string
	Num   int64
	Bulk  string
	Array []Value
}

// Constructors keep call sites short and typo-free.

func SimpleString(s string) Value { return Value{Typ: "string", Str: s} }
func Error(s string) Value        { return Value{Typ: "error", Str: s} }
func BulkString(s string) Value   { return Value{Typ: "bulk", Bulk: s} }
func Null() Value                 { return Value{Typ: "null"} }
func Integer(n int64) Value       { return Value{Typ: "integer", Num: n} }
func Array(vals []Value) Value    { return Value{Typ: "array", Array: vals} }

// Reading

type Reader struct {
	rd *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{rd: bufio.NewReader(r)}
}

// unexpected turns a plain EOF into ErrUnexpectedEOF. Use it whenever we are
// in the middle of a message, where running out of data is never "clean".
func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// readLine reads up to and including "\r\n" and returns the line without it.
func (r *Reader) readLine() ([]byte, error) {
	line, err := r.rd.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errors.New("protocol error: line not terminated with CRLF")
	}
	return line[:len(line)-2], nil
}

func (r *Reader) readInteger() (int, error) {
	line, err := r.readLine()
	if err != nil {
		return 0, unexpected(err)
	}
	n, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("protocol error: invalid integer %q", line)
	}
	return int(n), nil
}

// Read parses one RESP value. It returns io.EOF only if the stream ended
// cleanly before any byte of a new value was read.
func (r *Reader) Read() (Value, error) {
	t, err := r.rd.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch t {
	case prefixArray:
		return r.readArray()
	case prefixBulk:
		return r.readBulk()
	default:
		return Value{}, fmt.Errorf("protocol error: unknown type byte %q", t)
	}
}

func (r *Reader) readArray() (Value, error) {
	v := Value{Typ: "array"}

	n, err := r.readInteger()
	if err != nil {
		return v, err
	}
	if n < 0 || n > maxArrayLen {
		return v, fmt.Errorf("protocol error: invalid array length %d", n)
	}

	v.Array = make([]Value, 0, n)
	for i := 0; i < n; i++ {
		val, err := r.Read()
		if err != nil {
			return v, unexpected(err)
		}
		v.Array = append(v.Array, val)
	}

	return v, nil
}

func (r *Reader) readBulk() (Value, error) {
	v := Value{Typ: "bulk"}

	n, err := r.readInteger()
	if err != nil {
		return v, err
	}

	// "$-1\r\n" is the RESP null bulk string.
	if n == -1 {
		return Null(), nil
	}
	if n < 0 || n > maxBulkLen {
		return v, fmt.Errorf("protocol error: invalid bulk length %d", n)
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(r.rd, buf); err != nil {
		return v, unexpected(err)
	}
	v.Bulk = string(buf)

	crlf := make([]byte, 2)
	if _, err := io.ReadFull(r.rd, crlf); err != nil {
		return v, unexpected(err)
	}
	if crlf[0] != '\r' || crlf[1] != '\n' {
		return v, errors.New("protocol error: bulk string not terminated with CRLF")
	}

	return v, nil
}

// ---------- Writing ----------

type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

func (w *Writer) Write(v Value) error {
	_, err := w.w.Write(v.Marshal())
	return err
}

// Marshal converts a Value to RESP bytes.
func (v Value) Marshal() []byte {
	switch v.Typ {
	case "array":
		return v.marshalArray()
	case "bulk":
		return v.marshalBulk()
	case "string":
		return v.marshalString()
	case "null":
		return []byte("$-1\r\n")
	case "error":
		return v.marshalError()
	case "integer":
		return v.marshalInteger()
	default:
		return []byte{}
	}
}

func (v Value) marshalInteger() []byte {
	b := []byte{prefixInteger}
	b = strconv.AppendInt(b, v.Num, 10)
	return append(b, '\r', '\n')
}

func (v Value) marshalString() []byte {
	var b []byte
	b = append(b, prefixString)
	b = append(b, v.Str...)
	b = append(b, '\r', '\n')
	return b
}

func (v Value) marshalError() []byte {
	var b []byte
	b = append(b, prefixError)
	b = append(b, v.Str...)
	b = append(b, '\r', '\n')
	return b
}

func (v Value) marshalBulk() []byte {
	var b []byte
	b = append(b, prefixBulk)
	b = append(b, strconv.Itoa(len(v.Bulk))...)
	b = append(b, '\r', '\n')
	b = append(b, v.Bulk...)
	b = append(b, '\r', '\n')
	return b
}

func (v Value) marshalArray() []byte {
	var b []byte
	b = append(b, prefixArray)
	b = append(b, strconv.Itoa(len(v.Array))...)
	b = append(b, '\r', '\n')
	for _, item := range v.Array {
		b = append(b, item.Marshal()...)
	}
	return b
}
