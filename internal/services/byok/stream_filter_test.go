package byok

import (
	"strings"
	"testing"
)

func TestStreamSanitizerFiltersWholeTags(t *testing.T) {
	s := &StreamSanitizer{}
	out := s.Feed("Halo! [BOND_DAMAGE] apa kabar? [Add Exp: 50]")
	out += s.Flush()
	if strings.Contains(out, "BOND_DAMAGE") || strings.Contains(out, "Add Exp") {
		t.Fatalf("bond tags leaked: %q", out)
	}
	if !strings.Contains(out, "Halo!") || !strings.Contains(out, "apa kabar?") {
		t.Fatalf("legit text lost: %q", out)
	}
}

func TestStreamSanitizerHoldsSplitTag(t *testing.T) {
	s := &StreamSanitizer{}
	first := s.Feed("Halo [BOND_DA")
	if strings.Contains(first, "BOND") {
		t.Fatalf("split tag prefix leaked early: %q", first)
	}
	second := s.Feed("MAGE] dunia")
	combined := first + second + s.Flush()
	if strings.Contains(combined, "BOND") {
		t.Fatalf("bond tag leaked after join: %q", combined)
	}
	if !strings.Contains(combined, "Halo") || !strings.Contains(combined, "dunia") {
		t.Fatalf("legit text lost: %q", combined)
	}
}

func TestStreamSanitizerHoldsUnclosedThink(t *testing.T) {
	s := &StreamSanitizer{}
	first := s.Feed("Halo <think>rahasia")
	if strings.Contains(first, "rahasia") {
		t.Fatalf("unclosed think leaked: %q", first)
	}
	second := s.Feed(" banget</think> dunia")
	combined := first + second + s.Flush()
	if strings.Contains(combined, "rahasia") {
		t.Fatalf("think content leaked: %q", combined)
	}
	if !strings.Contains(combined, "Halo") || !strings.Contains(combined, "dunia") {
		t.Fatalf("legit text lost: %q", combined)
	}
}

func TestStreamSanitizerNoDuplicateEmit(t *testing.T) {
	s := &StreamSanitizer{}
	a := s.Feed("Halo ")
	b := s.Feed("dunia!")
	combined := a + b + s.Flush()
	if combined != "Halo dunia!" {
		t.Fatalf("unexpected output: %q", combined)
	}
}
