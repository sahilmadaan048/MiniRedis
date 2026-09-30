package server

import (
	"bufio"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sahilmadaan048/miniredis/internal/persistence"
	"github.com/sahilmadaan048/miniredis/internal/store"
)

// start launches a server backed by the AOF at path and returns its address
// and a stop function.
func start(t *testing.T, path string) (addr string, stop func()) {
	t.Helper()

	aof, err := persistence.NewAof(path)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(store.New(), aof)
	if err := srv.Load(); err != nil {
		t.Fatal(err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(l)

	return l.Addr().String(), func() {
		l.Close()
		aof.Close()
	}
}

func dial(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	return conn, bufio.NewReader(conn)
}

func encode(args ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return b.String()
}

// readReply reads one reply and returns it as raw RESP text.
func readReply(t *testing.T, br *bufio.Reader) string {
	t.Helper()

	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(line, "$") && line != "$-1\r\n" {
		data, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line += data
	}
	return line
}

func do(t *testing.T, conn net.Conn, br *bufio.Reader, args ...string) string {
	t.Helper()

	if _, err := conn.Write([]byte(encode(args...))); err != nil {
		t.Fatal(err)
	}
	return readReply(t, br)
}

func expect(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBasicCommands(t *testing.T) {
	addr, stop := start(t, filepath.Join(t.TempDir(), "t.aof"))
	defer stop()
	conn, br := dial(t, addr)

	expect(t, do(t, conn, br, "PING"), "+PONG\r\n")
	expect(t, do(t, conn, br, "SET", "name", "sahil"), "+OK\r\n")
	expect(t, do(t, conn, br, "get", "name"), "$5\r\nsahil\r\n") // lowercase works
	expect(t, do(t, conn, br, "GET", "missing"), "$-1\r\n")
	expect(t, do(t, conn, br, "HSET", "h", "f", "v"), "+OK\r\n")
	expect(t, do(t, conn, br, "HGET", "h", "f"), "$1\r\nv\r\n")
}

func TestErrorReplies(t *testing.T) {
	addr, stop := start(t, filepath.Join(t.TempDir(), "t.aof"))
	defer stop()
	conn, br := dial(t, addr)

	expect(t, do(t, conn, br, "FOO"), "-ERR unknown command 'FOO'\r\n")
	expect(t, do(t, conn, br, "SET", "a"), "-ERR wrong number of arguments for 'set' command\r\n")
	// The connection must still be usable after errors.
	expect(t, do(t, conn, br, "PING"), "+PONG\r\n")
}

func TestPipelining(t *testing.T) {
	addr, stop := start(t, filepath.Join(t.TempDir(), "t.aof"))
	defer stop()
	conn, br := dial(t, addr)

	// Three commands in a single TCP write.
	payload := encode("SET", "a", "1") + encode("GET", "a") + encode("PING")
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}

	expect(t, readReply(t, br), "+OK\r\n")
	expect(t, readReply(t, br), "$1\r\n1\r\n")
	expect(t, readReply(t, br), "+PONG\r\n")
}

func TestMultipleClients(t *testing.T) {
	addr, stop := start(t, filepath.Join(t.TempDir(), "t.aof"))
	defer stop()

	c1, br1 := dial(t, addr)
	c2, br2 := dial(t, addr)

	expect(t, do(t, c1, br1, "SET", "shared", "x"), "+OK\r\n")
	expect(t, do(t, c2, br2, "GET", "shared"), "$1\r\nx\r\n")

	// Closing one client must not affect the other.
	c1.Close()
	expect(t, do(t, c2, br2, "PING"), "+PONG\r\n")
}

func TestConcurrentClients(t *testing.T) {
	addr, stop := start(t, filepath.Join(t.TempDir(), "t.aof"))
	defer stop()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			br := bufio.NewReader(conn)

			key := fmt.Sprintf("key%d", i)
			for j := 0; j < 50; j++ {
				conn.Write([]byte(encode("SET", key, "v")))
				if r, _ := br.ReadString('\n'); r != "+OK\r\n" {
					t.Errorf("client %d: got %q", i, r)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestPersistenceAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.aof")

	addr, stop := start(t, path)
	conn, br := dial(t, addr)
	expect(t, do(t, conn, br, "SET", "name", "sahil"), "+OK\r\n")
	expect(t, do(t, conn, br, "HSET", "user", "city", "srinagar"), "+OK\r\n")
	do(t, conn, br, "SET", "bad") // failed command must not be persisted
	stop()

	addr2, stop2 := start(t, path)
	defer stop2()
	conn2, br2 := dial(t, addr2)

	expect(t, do(t, conn2, br2, "GET", "name"), "$5\r\nsahil\r\n")
	expect(t, do(t, conn2, br2, "HGET", "user", "city"), "$8\r\nsrinagar\r\n")
	expect(t, do(t, conn2, br2, "GET", "bad"), "$-1\r\n")
}