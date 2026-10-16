package services

// mem_store.go — in-memory fallback store untuk cooldown, pending queue,
// dan streaming Pub/Sub saat Redis tidak tersedia.
// Semua struktur ini thread-safe dan diinisialisasi saat package di-load.

import (
	"context"
	"sync"
	"time"
)

// ─── Cooldown store ───────────────────────────────────────────────────────────

type memCooldownStore struct {
	mu    sync.Mutex
	items map[string]time.Time // key → expiry
}

func (s *memCooldownStore) Set(key string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = time.Now().Add(ttl)
}

func (s *memCooldownStore) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.items[key]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.items, key)
		return false
	}
	return true
}

var memCooldown = &memCooldownStore{items: make(map[string]time.Time)}

// ─── Pending queue store ─────────────────────────────────────────────────────

type memPendingStore struct {
	mu     sync.Mutex
	queues map[string][]string // key → FIFO list of UUIDs
}

func (s *memPendingStore) Push(key, uuid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queues[key] = append(s.queues[key], uuid)
}

func (s *memPendingStore) Pop(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queues[key]
	if len(q) == 0 {
		return ""
	}
	val := q[0]
	s.queues[key] = q[1:]
	return val
}

var memPending = &memPendingStore{queues: make(map[string][]string)}

// ─── Streaming store (in-process Pub/Sub + buffer) ───────────────────────────

type memStreamEntry struct {
	subs   []chan string
	buffer []string // chronological, capped at streamBufMax
}

type memStreamStore struct {
	mu      sync.Mutex
	streams map[string]*memStreamEntry
}

func (s *memStreamStore) entry(uuid string) *memStreamEntry {
	e, ok := s.streams[uuid]
	if !ok {
		e = &memStreamEntry{}
		s.streams[uuid] = e
	}
	return e
}

// Publish mengirim delta ke semua subscriber aktif dan tambahkan ke buffer.
func (s *memStreamStore) Publish(uuid, msg string) {
	s.mu.Lock()
	e := s.entry(uuid)
	// Append to buffer (capped)
	if msg != "[DONE]" {
		e.buffer = append(e.buffer, msg)
		if len(e.buffer) > streamBufMax {
			e.buffer = e.buffer[len(e.buffer)-streamBufMax:]
		}
	}
	// Snapshot subs agar kirim di luar lock
	subs := make([]chan string, len(e.subs))
	copy(subs, e.subs)
	s.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
			// channel penuh / subscriber lambat — skip
		}
	}

	// Bersihkan entry setelah [DONE]
	if msg == "[DONE]" {
		go func() {
			time.Sleep(streamBufTTL)
			s.mu.Lock()
			delete(s.streams, uuid)
			s.mu.Unlock()
		}()
	}
}

// GetBuffer mengembalikan salinan buffer chronological.
func (s *memStreamStore) GetBuffer(uuid string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.streams[uuid]
	if !ok {
		return nil
	}
	out := make([]string, len(e.buffer))
	copy(out, e.buffer)
	return out
}

// Subscribe mendaftarkan ch sebagai subscriber untuk uuid.
// Goroutine internal mem-forward pesan ke ch hingga ctx selesai atau "[DONE]".
// ch dikembalikan langsung agar caller dapat select padanya.
func (s *memStreamStore) Subscribe(ctx context.Context, uuid string, ch chan string) <-chan string {
	s.mu.Lock()
	e := s.entry(uuid)
	e.subs = append(e.subs, ch)
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		// Hapus ch dari daftar subscriber
		s.mu.Lock()
		if entry, ok := s.streams[uuid]; ok {
			for i, sub := range entry.subs {
				if sub == ch {
					entry.subs = append(entry.subs[:i], entry.subs[i+1:]...)
					break
				}
			}
		}
		s.mu.Unlock()
		close(ch)
	}()

	return ch
}

var memStream = &memStreamStore{streams: make(map[string]*memStreamEntry)}
