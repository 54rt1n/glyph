package id

import "testing"

func TestNewFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		got := New(2)
		if !Valid(got) {
			t.Fatalf("New(2) = %q, not a valid id", got)
		}
		if len(got) != 6 {
			t.Fatalf("New(2) = %q, want g-xxxx", got)
		}
	}
}

func TestGenerateAvoidsCollisions(t *testing.T) {
	taken := map[string]bool{}
	for i := 0; i < 500; i++ {
		got := Generate(func(s string) bool { return taken[s] })
		if taken[got] {
			t.Fatalf("Generate returned taken id %q", got)
		}
		taken[got] = true
	}
}

func TestGenerateLengthensUnderPressure(t *testing.T) {
	// exists always true for short ids forces the longer fallback
	got := Generate(func(s string) bool { return len(s) < 10 })
	if !Valid(got) || len(got) < 10 {
		t.Fatalf("Generate under pressure = %q, want longer id", got)
	}
}
