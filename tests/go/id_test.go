package tests

import (
	"encoding/base64"
	"testing"

	"github.com/yhakami/bincrypt/src/utils"
)

func TestGenerateID(t *testing.T) {
	// Test ID generation
	id1, err := utils.GenerateID()
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}

	// Check ID length (32 bytes base64 encoded without padding = 43 chars)
	if len(id1) != 43 {
		t.Errorf("Expected ID length 43, got %d", len(id1))
	}

	// Verify it's valid base64
	decoded, err := base64.RawURLEncoding.DecodeString(id1)
	if err != nil {
		t.Errorf("Failed to decode ID: %v", err)
	}

	// Verify entropy (32 bytes = 256 bits)
	if len(decoded) != 32 {
		t.Errorf("Expected 32 bytes of entropy, got %d", len(decoded))
	}

	// Test uniqueness (generate multiple IDs)
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, err := utils.GenerateID()
		if err != nil {
			t.Fatalf("Failed to generate ID on iteration %d: %v", i, err)
		}

		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	// Verify IDs are URL-safe (no special chars that need encoding)
	for id := range ids {
		for _, c := range id {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_') {
				t.Errorf("ID contains non-URL-safe character: %c in ID %s", c, id)
			}
		}
	}
}

func TestGenerateIDEntropy(t *testing.T) {
	// Statistical test for randomness
	// Generate many IDs and check bit distribution
	bitCounts := make([]int, 256) // 32 bytes * 8 bits
	iterations := 1000

	for i := 0; i < iterations; i++ {
		id, err := utils.GenerateID()
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		decoded, _ := base64.RawURLEncoding.DecodeString(id)

		// Count bits
		for byteIdx, b := range decoded {
			for bitIdx := 0; bitIdx < 8; bitIdx++ {
				if b&(1<<uint(bitIdx)) != 0 {
					bitCounts[byteIdx*8+bitIdx]++
				}
			}
		}
	}

	// Check that each bit position is roughly 50% set
	// Allow 40-60% range for randomness
	for bitPos, count := range bitCounts {
		percentage := float64(count) / float64(iterations) * 100
		if percentage < 40 || percentage > 60 {
			t.Errorf("Bit position %d has suspicious distribution: %.1f%% set", bitPos, percentage)
		}
	}
}
