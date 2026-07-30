// Package id generates glyph ids: g- followed by lowercase hex (g-a1b2).
package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

var pattern = regexp.MustCompile(`^g-[0-9a-f]{4,12}$`)

// New returns a fresh id with n random bytes (2 bytes → g-xxxx).
func New(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("id: crypto/rand failed: %v", err))
	}
	return "g-" + hex.EncodeToString(b)
}

// Valid reports whether s looks like a glyph id.
func Valid(s string) bool {
	return pattern.MatchString(s)
}

// Generate returns an unused id, consulting exists. It starts at 4 hex chars
// and lengthens if the short space is crowded.
func Generate(exists func(string) bool) string {
	for _, n := range []int{2, 2, 2, 3, 3, 4, 6} {
		id := New(n)
		if !exists(id) {
			return id
		}
	}
	// 6 random bytes twice colliding is practically impossible.
	return New(6)
}

// Edge returns an edge id (e- prefix, 4 random bytes).
func Edge() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("id: crypto/rand failed: %v", err))
	}
	return "e-" + hex.EncodeToString(b)
}
