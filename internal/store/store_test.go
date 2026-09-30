package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestSetGet(t *testing.T) {
	s := New()

	if _, ok := s.Get("missing"); ok {
		t.Fatal("missing key reported as present")
	}

	s.Set("k", "v1")
	s.Set("k", "v2") // overwrite
	if v, ok := s.Get("k"); !ok || v != "v2" {
		t.Fatalf("got (%q, %v), want (v2, true)", v, ok)
	}

	// An empty value is different from a missing key.
	s.Set("empty", "")
	if v, ok := s.Get("empty"); !ok || v != "" {
		t.Fatalf("got (%q, %v), want (\"\", true)", v, ok)
	}
}

func TestHSetHGet(t *testing.T) {
	s := New()

	if _, ok := s.HGet("nohash", "f"); ok {
		t.Fatal("field of a missing hash reported as present")
	}

	s.HSet("user:1", "city", "srinagar")
	if v, ok := s.HGet("user:1", "city"); !ok || v != "srinagar" {
		t.Fatalf("got (%q, %v)", v, ok)
	}
	if _, ok := s.HGet("user:1", "other"); ok {
		t.Fatal("missing field reported as present")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := New()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i)
			for j := 0; j < 200; j++ {
				s.Set(key, "v")
				s.Get(key)
				s.HSet("h", key, "v")
				s.HGet("h", key)
			}
		}(i)
	}

	wg.Wait()
}