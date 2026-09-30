package commands

import (
	"testing"

	"github.com/sahilmadaan048/miniredis/internal/resp"
	"github.com/sahilmadaan048/miniredis/internal/store"
)

// run executes a command by name against s, the same way the server does.
func run(t *testing.T, s *store.Store, name string, args ...string) resp.Value {
	t.Helper()

	cmd, ok := Lookup(name)
	if !ok {
		t.Fatalf("command %q not found", name)
	}

	vals := make([]resp.Value, len(args))
	for i, a := range args {
		vals[i] = resp.BulkString(a)
	}
	return cmd.Handler(s, vals)
}

func TestLookup(t *testing.T) {
	for _, name := range []string{"SET", "set", "SeT"} {
		if _, ok := Lookup(name); !ok {
			t.Errorf("Lookup(%q) failed", name)
		}
	}
	if _, ok := Lookup("NOPE"); ok {
		t.Error("Lookup(NOPE) should fail")
	}
}

func TestWriteFlags(t *testing.T) {
	want := map[string]bool{"SET": true, "HSET": true, "GET": false, "HGET": false, "PING": false}
	for name, w := range want {
		cmd, _ := Lookup(name)
		if cmd.Write != w {
			t.Errorf("%s: Write = %v, want %v", name, cmd.Write, w)
		}
	}
}

func TestPing(t *testing.T) {
	s := store.New()

	if got := run(t, s, "PING"); got.Typ != "string" || got.Str != "PONG" {
		t.Fatalf("got %+v", got)
	}
	if got := run(t, s, "PING", "hello"); got.Str != "hello" {
		t.Fatalf("got %+v", got)
	}
}

func TestSetGet(t *testing.T) {
	s := store.New()

	if got := run(t, s, "SET", "name", "sahil"); got.Typ != "string" || got.Str != "OK" {
		t.Fatalf("SET: got %+v", got)
	}
	if got := run(t, s, "GET", "name"); got.Typ != "bulk" || got.Bulk != "sahil" {
		t.Fatalf("GET: got %+v", got)
	}
	if got := run(t, s, "GET", "missing"); got.Typ != "null" {
		t.Fatalf("GET missing: got %+v", got)
	}
}

func TestHSetHGet(t *testing.T) {
	s := store.New()

	run(t, s, "HSET", "user:1", "city", "srinagar")

	if got := run(t, s, "HGET", "user:1", "city"); got.Bulk != "srinagar" {
		t.Fatalf("got %+v", got)
	}
	if got := run(t, s, "HGET", "user:1", "nope"); got.Typ != "null" {
		t.Fatalf("got %+v", got)
	}
}

func TestWrongArgumentCount(t *testing.T) {
	s := store.New()

	tests := []struct {
		name string
		args []string
	}{
		{"SET", []string{"a"}},
		{"SET", []string{"a", "b", "c"}},
		{"GET", nil},
		{"GET", []string{"a", "b"}},
		{"HSET", []string{"h", "f"}},
		{"HGET", []string{"h"}},
	}

	for _, tt := range tests {
		if got := run(t, s, tt.name, tt.args...); got.Typ != "error" {
			t.Errorf("%s %v: want error, got %+v", tt.name, tt.args, got)
		}
	}
}