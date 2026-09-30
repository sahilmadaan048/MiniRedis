// Package persistence implements the append-only file (AOF).
package persistence

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/sahilmadaan048/miniredis/internal/resp"
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
	close(aof.done)

	aof.mu.Lock()
	defer aof.mu.Unlock()

	aof.file.Sync()
	return aof.file.Close()
}

func (aof *Aof) Write(value resp.Value) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	_, err := aof.file.Write(value.Marshal())
	return err
}

// Read replays every command in the file, calling callback for each one.
func (aof *Aof) Read(callback func(value resp.Value)) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	if _, err := aof.file.Seek(0, 0); err != nil {
		return err
	}

	rd := resp.NewReader(aof.file)

	for {
		value, err := rd.Read()
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