package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	STRING  = '+'
	ERROR   = '-'
	INTEGER = ':'
	BULK    = '$'
	ARRAY   = '*'
)

// Limits protect the server from clients that announce absurd sizes.
const (
	maxBulkLen  = 512 * 1024 * 1024 // 512 MB, same as Redis
	maxArrayLen = 1024 * 1024
)

type Value struct {
	typ   string
	str   string
	num   int
	bulk  string
	array []Value
}

type Resp struct {
	reader *bufio.Reader
}

type Writer struct {
	writer io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: w}
}

func (w *Writer) Write(v Value) error {
	_, err := w.writer.Write(v.Marshal())
	return err
}

func NewResp(rd io.Reader) *Resp {
	return &Resp{reader: bufio.NewReader(rd)}
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
func (r *Resp) readLine() ([]byte, error) {
	line, err := r.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errors.New("protocol error: line not terminated with CRLF")
	}
	return line[:len(line)-2], nil
}

func (r *Resp) readInteger() (int, error) {
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
func (r *Resp) Read() (Value, error) {
	t, err := r.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch t {
	case ARRAY:
		return r.readArray()
	case BULK:
		return r.readBulk()
	default:
		return Value{}, fmt.Errorf("protocol error: unknown type byte %q", t)
	}
}

func (r *Resp) readArray() (Value, error) {
	v := Value{typ: "array"}

	n, err := r.readInteger()
	if err != nil {
		return v, err
	}
	if n < 0 || n > maxArrayLen {
		return v, fmt.Errorf("protocol error: invalid array length %d", n)
	}

	v.array = make([]Value, 0, n)
	for i := 0; i < n; i++ {
		val, err := r.Read()
		if err != nil {
			return v, unexpected(err)
		}
		v.array = append(v.array, val)
	}

	return v, nil
}

func (r *Resp) readBulk() (Value, error) {
	v := Value{typ: "bulk"}

	n, err := r.readInteger()
	if err != nil {
		return v, err
	}

	// "$-1\r\n" is the RESP null bulk string.
	if n == -1 {
		return Value{typ: "null"}, nil
	}
	if n < 0 || n > maxBulkLen {
		return v, fmt.Errorf("protocol error: invalid bulk length %d", n)
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(r.reader, buf); err != nil {
		return v, unexpected(err)
	}
	v.bulk = string(buf)

	// Every bulk string is followed by "\r\n".
	crlf := make([]byte, 2)
	if _, err := io.ReadFull(r.reader, crlf); err != nil {
		return v, unexpected(err)
	}
	if crlf[0] != '\r' || crlf[1] != '\n' {
		return v, errors.New("protocol error: bulk string not terminated with CRLF")
	}

	return v, nil
}

// Marshal converts a Value to RESP bytes.
func (v Value) Marshal() []byte {
	switch v.typ {
	case "array":
		return v.marshalArray()
	case "bulk":
		return v.marshalBulk()
	case "string":
		return v.marshalString()
	case "null":
		return v.marshalNull()
	case "error":
		return v.marshalError()
	default:
		return []byte{}
	}
}

func (v Value) marshalString() []byte {
	var bytes []byte
	bytes = append(bytes, STRING)
	bytes = append(bytes, v.str...)
	bytes = append(bytes, '\r', '\n')
	return bytes
}

func (v Value) marshalBulk() []byte {
	var bytes []byte
	bytes = append(bytes, BULK)
	bytes = append(bytes, strconv.Itoa(len(v.bulk))...)
	bytes = append(bytes, '\r', '\n')
	bytes = append(bytes, v.bulk...)
	bytes = append(bytes, '\r', '\n')
	return bytes
}

func (v Value) marshalArray() []byte {
	n := len(v.array)
	var bytes []byte
	bytes = append(bytes, ARRAY)
	bytes = append(bytes, strconv.Itoa(n)...)
	bytes = append(bytes, '\r', '\n')
	for i := 0; i < n; i++ {
		bytes = append(bytes, v.array[i].Marshal()...)
	}
	return bytes
}

func (v Value) marshalError() []byte {
	var bytes []byte
	bytes = append(bytes, ERROR)
	bytes = append(bytes, v.str...)
	bytes = append(bytes, '\r', '\n')
	return bytes
}

func (v Value) marshalNull() []byte {
	return []byte("$-1\r\n")
}
