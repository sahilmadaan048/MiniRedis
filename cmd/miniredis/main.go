package main

import (
	"fmt"
	"net"
	"os"

	"github.com/sahilmadaan048/miniredis/internal/persistence"
	"github.com/sahilmadaan048/miniredis/internal/server"
	"github.com/sahilmadaan048/miniredis/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run is separate from main so that deferred cleanups (like aof.Close)
// still run when we return an error; os.Exit skips defers.
func run() error {
	aof, err := persistence.NewAof("database.aof")
	if err != nil {
		return err
	}
	defer aof.Close()

	srv := server.New(store.New(), aof)

	if err := srv.Load(); err != nil {
		return fmt.Errorf("loading AOF: %w", err)
	}

	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		return err
	}
	defer l.Close()

	fmt.Println("Listening on port :6379")
	return srv.Serve(l)
}