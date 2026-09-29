package main

import (
	"fmt"
	"io"
	"net"
	"strings"
)

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
			fmt.Println("Invalid command:", command)
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

	// One reader and one writer for the whole lifetime of the connection.
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

		if value.typ != "array" {
			fmt.Println("Invalid request, expected array")
			continue
		}

		if len(value.array) == 0 {
			fmt.Println("Invalid request, expected array length > 0")
			continue
		}

		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]

		handler, ok := Handlers[command]
		if !ok {
			fmt.Println("Invalid command:", command)
			writer.Write(Value{typ: "string", str: ""})
			continue
		}

		// Persist write commands
		if command == "SET" || command == "HSET" {
			if err := aof.Write(value); err != nil {
				fmt.Println("Error writing to AOF:", err)
				continue
			}
		}

		result := handler(args)
		writer.Write(result)
	}
}