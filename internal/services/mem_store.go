package services

// mem_store.go — in-memory fallback store untuk streaming Pub/Sub saat Redis
// tidak tersedia. Thread-safe, diinisialisasi saat package di-load.

import (
	"context"
	"sync"
	"time"
)

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
