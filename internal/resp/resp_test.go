package resp

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestReadCommand(t *testing.T) {
	in := "*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$5\r\nsahil\r\n"

	got, err := NewReader(strings.NewReader(in)).Read()
	if err != nil {
		t.Fatal(err)
	}

	want := Value{Typ: "array", Array: []Value{
		BulkString("SET"), BulkString("name"), BulkString("sahil"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReadNullBulk(t *testing.T) {
	got, err := NewReader(strings.NewReader("$-1\r\n")).Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Typ != "null" {
		t.Fatalf("got %+v, want null", got)
	}
}

func TestReadPipelined(t *testing.T) {
	in := "*1\r\n$4\r\nPING\r\n*1\r\n$4\r\nPING\r\n"
	rd := NewReader(strings.NewReader(in))

	for i := 0; i < 2; i++ {
		if _, err := rd.Read(); err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
	}
	if _, err := rd.Read(); err != io.EOF {
		t.Fatalf("want io.EOF after last command, got %v", err)
	}
}

func TestReadEOFSemantics(t *testing.T) {
	// Clean end of stream between commands.
	if _, err := NewReader(strings.NewReader("")).Read(); err != io.EOF {
		t.Fatalf("empty input: want io.EOF, got %v", err)
	}

	// Stream cut off in the middle of a command.
	truncated := []string{
		"*3\r\n$3\r\nSET\r\n",
		"*1\r\n$5\r\nhel",
		"*2\r\n",
	}
	for _, in := range truncated {
		_, err := NewReader(strings.NewReader(in)).Read()
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("%q: want ErrUnexpectedEOF, got %v", in, err)
		}
	}
}

func TestReadProtocolErrors(t *testing.T) {
	bad := []string{
		"?garbage\r\n",           // unknown type byte
		"*abc\r\n",               // non-numeric length
		"*-5\r\n",                // negative array length
		"$3\r\nabcXX",            // bulk not terminated by CRLF
		"$999999999999\r\nabc\r\n", // absurd bulk length
		"*1\n",                   // LF without CR
	}
	for _, in := range bad {
		if _, err := NewReader(strings.NewReader(in)).Read(); err == nil {
			t.Errorf("%q: expected an error, got nil", in)
		}
	}
}

func TestMarshal(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want string
	}{
		{"simple string", SimpleString("OK"), "+OK\r\n"},
		{"error", Error("ERR boom"), "-ERR boom\r\n"},
		{"bulk", BulkString("hi"), "$2\r\nhi\r\n"},
		{"empty bulk", BulkString(""), "$0\r\n\r\n"},
		{"null", Null(), "$-1\r\n"},
		{"array", Value{Typ: "array", Array: []Value{BulkString("GET"), BulkString("k")}},
			"*2\r\n$3\r\nGET\r\n$1\r\nk\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.v.Marshal()); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	orig := Value{Typ: "array", Array: []Value{
		BulkString("SET"), BulkString("k"), BulkString("value with\r\nnewline"),
	}}

	got, err := NewReader(strings.NewReader(string(orig.Marshal()))).Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, orig) {
		t.Fatalf("got %+v, want %+v", got, orig)
	}
}