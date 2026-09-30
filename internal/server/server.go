// Package server accepts TCP connections and runs commands against the store.
package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/sahilmadaan048/miniredis/internal/commands"
	"github.com/sahilmadaan048/miniredis/internal/persistence"
	"github.com/sahilmadaan048/miniredis/internal/resp"
	"github.com/sahilmadaan048/miniredis/internal/store"
)

type Server struct {
	store *store.Store
	aof   *persistence.Aof

	// writeMu makes "execute + append to AOF" atomic, so the order of commands
	// in the file always matches the order in which they were applied in memory.
	writeMu sync.Mutex
}

func New(st *store.Store, aof *persistence.Aof) *Server {
	return &Server{store: st, aof: aof}
}

// Load replays the AOF into the store. Call it before Serve.
func (s *Server) Load() error {
	return s.aof.Read(func(v resp.Value) {
		if v.Typ != "array" || len(v.Array) == 0 {
			return
		}

		cmd, ok := commands.Lookup(v.Array[0].Bulk)
		if !ok {
			fmt.Println("Invalid command in AOF:", v.Array[0].Bulk)
			return
		}

		cmd.Handler(s.store, v.Array[1:])
	})
}

// Serve accepts connections on l until l is closed.
func (s *Server) Serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			fmt.Println("Accept error:", err)
			continue
		}

		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	rd := resp.NewReader(conn)
	wr := resp.NewWriter(conn)

	for {
		value, err := rd.Read()
		if err != nil {
			if err != io.EOF {
				fmt.Println("Read error:", err)
			}
			return
		}

		if err := wr.Write(s.execute(value)); err != nil {
			return
		}
	}
}

// execute runs one request and returns the reply. Successful write commands
// are appended to the AOF after they are applied.
func (s *Server) execute(v resp.Value) resp.Value {
	if v.Typ != "array" || len(v.Array) == 0 {
		return resp.Error("ERR invalid request, expected non-empty array")
	}

	name := v.Array[0].Bulk
	cmd, ok := commands.Lookup(name)
	if !ok {
		return resp.Error(fmt.Sprintf("ERR unknown command '%s'", name))
	}

	args := v.Array[1:]

	if !cmd.Write {
		return cmd.Handler(s.store, args)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	result := cmd.Handler(s.store, args)

	if result.Typ != "error" {
		if err := s.aof.Write(v); err != nil {
			fmt.Println("Error writing to AOF:", err)
			return resp.Error("ERR failed to persist command")
		}
	}

	return result
}