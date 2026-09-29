package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

// Commands that modify data and therefore must be persisted.
var writeCommands = map[string]bool{
	"SET":  true,
	"HSET": true,
}

// writeMu makes "execute + append to AOF" atomic, so the order of commands in
// the file always matches the order in which they were applied in memory.
var writeMu sync.Mutex

func main() {
	aof, err := NewAof("database.aof")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer aof.Close()

	// Load existing data from AOF before accepting any client.
	err = aof.Read(func(value Value) {
		if value.typ != "array" || len(value.array) == 0 {
			return
		}

		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]

		handler, ok := Handlers[command]
		if !ok {
			fmt.Println("Invalid command in AOF:", command)
			return
		}

		handler(args)
	})
	if err != nil {
		fmt.Println("Error reading AOF:", err)
		return
	}

	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	fmt.Println("Listening on port :6379")

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}

		go handleConn(conn, aof)
	}
}

func handleConn(conn net.Conn, aof *Aof) {
	defer conn.Close()

	resp := NewResp(conn)
	writer := NewWriter(conn)

	for {
		value, err := resp.Read()
		if err != nil {
			if err != io.EOF {
				fmt.Println("Read error:", err)
			}
			return
		}

		if value.typ != "array" || len(value.array) == 0 {
			if err := writer.Write(Value{typ: "error", str: "ERR invalid request, expected non-empty array"}); err != nil {
				return
			}
			continue
		}

		name := value.array[0].bulk
		command := strings.ToUpper(name)
		args := value.array[1:]

		handler, ok := Handlers[command]
		if !ok {
			errReply := Value{typ: "error", str: fmt.Sprintf("ERR unknown command '%s'", name)}
			if err := writer.Write(errReply); err != nil {
				return
			}
			continue
		}

		var result Value

		if writeCommands[command] {
			writeMu.Lock()
			result = handler(args)

			// Only successful commands are logged.
			if result.typ != "error" {
				if err := aof.Write(value); err != nil {
					fmt.Println("Error writing to AOF:", err)
					result = Value{typ: "error", str: "ERR failed to persist command"}
				}
			}
			writeMu.Unlock()
		} else {
			result = handler(args)
		}

		if err := writer.Write(result); err != nil {
			return
		}
	}
}
