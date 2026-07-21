package models

import (
	"testing"
	"time"
)

func TestHasActiveBan_NotBanned(t *testing.T) {
	u := User{IsBanned: false}
	if u.HasActiveBan() {
		t.Error("expected false for non-banned user")
	}
}

func TestHasActiveBan_BannedNoExpiry(t *testing.T) {
	u := User{IsBanned: true, BannedUntil: nil}
	if !u.HasActiveBan() {
		t.Error("expected true for banned user without expiry")
	}
}

func TestHasActiveBan_BannedExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	u := User{IsBanned: true, BannedUntil: &past}
	if u.HasActiveBan() {
		t.Error("expected false for banned user with expired ban")
	}
}

func TestHasActiveBan_BannedActive(t *testing.T) {
	future := time.Now().Add(1 * time.Hour)
	u := User{IsBanned: true, BannedUntil: &future}
	if !u.HasActiveBan() {
		t.Error("expected true for banned user with active ban")
	}
}
