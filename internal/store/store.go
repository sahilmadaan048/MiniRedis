// Package store holds the in-memory data and is safe for concurrent use.
package store

import "sync"

type Store struct {
	mu     sync.RWMutex
	strs   map[string]string
	hashes map[string]map[string]string
}

func New() *Store {
	return &Store{
		strs:   make(map[string]string),
		hashes: make(map[string]map[string]string),
	}
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	s.strs[key] = value
	s.mu.Unlock()
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	v, ok := s.strs[key]
	s.mu.RUnlock()
	return v, ok
}

func (s *Store) HSet(hash, field, value string) {
	s.mu.Lock()
	h, ok := s.hashes[hash]
	if !ok {
		h = make(map[string]string)
		s.hashes[hash] = h
	}
	h[field] = value
	s.mu.Unlock()
}

func (s *Store) HGet(hash, field string) (string, bool) {
	s.mu.RLock()
	v, ok := s.hashes[hash][field] // reading a nil inner map is safe in Go
	s.mu.RUnlock()
	return v, ok
}