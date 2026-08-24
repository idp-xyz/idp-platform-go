package numbering

import (
	"errors"
	"testing"
)

func TestScrambleIsBijectiveOverDomain(t *testing.T) {
	seen := make(map[int64]bool, 100)
	for index := int64(0); index < 100; index++ {
		value, effWidth, err := Scramble(index, 10, 2, "k1", 6)
		if err != nil {
			t.Fatalf("scramble %d: %v", index, err)
		}
		if effWidth != 2 {
			t.Fatalf("width must stay 2 inside the domain, got %d", effWidth)
		}
		if value < 0 || value >= 100 {
			t.Fatalf("value %d escapes the domain", value)
		}
		if seen[value] {
			t.Fatalf("collision at index %d value %d", index, value)
		}
		seen[value] = true
	}
	if len(seen) != 100 {
		t.Fatalf("expected a full permutation, got %d values", len(seen))
	}
}

func TestScrambleWidensWhenCounterOutgrowsDomain(t *testing.T) {
	value, effWidth, err := Scramble(100, 10, 2, "k1", 6)
	if err != nil {
		t.Fatal(err)
	}
	if effWidth != 3 {
		t.Fatalf("expected effective width 3, got %d", effWidth)
	}
	if value < 0 || value >= 1000 {
		t.Fatalf("value %d escapes the widened domain", value)
	}
}

func TestScrambleIsDeterministicAndKeySensitive(t *testing.T) {
	first, _, err := Scramble(7, 10, 6, "key-a", 6)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := Scramble(7, 10, 6, "key-a", 6)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("same key must scramble identically: %d vs %d", first, second)
	}

	differs := false
	for index := int64(0); index < 100; index++ {
		a, _, err := Scramble(index, 10, 6, "key-a", 6)
		if err != nil {
			t.Fatal(err)
		}
		b, _, err := Scramble(index, 10, 6, "key-b", 6)
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			differs = true
			break
		}
	}
	if !differs {
		t.Fatal("different keys must produce different mappings")
	}
}

func TestScrambleBase36Domain(t *testing.T) {
	seen := make(map[int64]bool, 36)
	for index := int64(0); index < 36; index++ {
		value, effWidth, err := Scramble(index, 36, 1, "k", 6)
		if err != nil {
			t.Fatal(err)
		}
		if effWidth != 1 || value < 0 || value >= 36 {
			t.Fatalf("index %d: value %d width %d", index, value, effWidth)
		}
		if seen[value] {
			t.Fatalf("collision at %d", value)
		}
		seen[value] = true
	}
}

func TestScrambleRejectsOverflowingDomain(t *testing.T) {
	if _, _, err := Scramble(0, 10, 19, "k", 6); !errors.Is(err, ErrScrambleDomain) {
		t.Fatalf("expected ErrScrambleDomain, got %v", err)
	}
}
