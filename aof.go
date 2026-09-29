package main

import (
	"io"
	"os"
	"sync"
	"time"
)

type Aof struct {
	file *os.File
	mu   sync.Mutex
	done chan struct{}
}

func NewAof(path string) (*Aof, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	aof := &Aof{
		file: f,
		done: make(chan struct{}),
	}

	go aof.syncLoop()

	return aof, nil
}

// syncLoop flushes the file to disk once per second until Close is called.
func (aof *Aof) syncLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			aof.mu.Lock()
			aof.file.Sync()
			aof.mu.Unlock()
		case <-aof.done:
			return
		}
	}
}

func (aof *Aof) Close() error {
	close(aof.done) // stop the sync goroutine

	aof.mu.Lock()
	defer aof.mu.Unlock()

	aof.file.Sync() // final flush so nothing is left in the OS cache
	return aof.file.Close()
}

func (aof *Aof) Write(value Value) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	_, err := aof.file.Write(value.Marshal())
	return err
}

// Read replays every command in the file, calling callback for each one.
func (aof *Aof) Read(callback func(value Value)) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	// Start reading from the beginning of the file.
	if _, err := aof.file.Seek(0, 0); err != nil {
		return err
	}

	resp := NewResp(aof.file)

	for {
		value, err := resp.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		callback(value)
	}

	return nil
}
