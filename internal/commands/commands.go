// Package commands maps Redis command names to their implementations.
package commands

import (
	"strings"

	"github.com/sahilmadaan048/miniredis/internal/resp"
	"github.com/sahilmadaan048/miniredis/internal/store"
)

type Handler func(s *store.Store, args []resp.Value) resp.Value

type Command struct {
	Handler Handler
	Write   bool // true if the command modifies data and must be logged to the AOF
}

var table = map[string]Command{
	"PING": {Handler: ping},
	"SET":  {Handler: set, Write: true},
	"GET":  {Handler: get},
	"HSET": {Handler: hset, Write: true},
	"HGET": {Handler: hget},
}

// Lookup finds a command by name, case-insensitively.
func Lookup(name string) (Command, bool) {
	c, ok := table[strings.ToUpper(name)]
	return c, ok
}

func wrongArgs(cmd string) resp.Value {
	return resp.Error("ERR wrong number of arguments for '" + cmd + "' command")
}

func ping(_ *store.Store, args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.SimpleString("PONG")
	}
	return resp.SimpleString(args[0].Bulk)
}

func set(s *store.Store, args []resp.Value) resp.Value {
	if len(args) != 2 {
		return wrongArgs("set")
	}
	s.Set(args[0].Bulk, args[1].Bulk)
	return resp.SimpleString("OK")
}

func get(s *store.Store, args []resp.Value) resp.Value {
	if len(args) != 1 {
		return wrongArgs("get")
	}
	v, ok := s.Get(args[0].Bulk)
	if !ok {
		return resp.Null()
	}
	return resp.BulkString(v)
}

func hset(s *store.Store, args []resp.Value) resp.Value {
	if len(args) != 3 {
		return wrongArgs("hset")
	}
	s.HSet(args[0].Bulk, args[1].Bulk, args[2].Bulk)
	return resp.SimpleString("OK")
}

func hget(s *store.Store, args []resp.Value) resp.Value {
	if len(args) != 2 {
		return wrongArgs("hget")
	}
	v, ok := s.HGet(args[0].Bulk, args[1].Bulk)
	if !ok {
		return resp.Null()
	}
	return resp.BulkString(v)
}