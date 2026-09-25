package tui

import (
	"strings"
	"testing"
)

func TestCompositeOverlayBoundsSafety(t *testing.T) {
	bg := "Line1: 12345678901234567890\nLine2: 12345678901234567890\nLine3: 12345678901234567890"
	fg := "[Modal Box Header]\n[Modal Content Line]"

	// Test 1: Negative posX (Warp zoom expanding past left edge)
	res1 := compositeOverlay(bg, fg, -10, 0)
	if len(strings.Split(res1, "\n")) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(strings.Split(res1, "\n")))
	}

	// Test 2: Negative posY (Warp zoom expanding past top edge)
	res2 := compositeOverlay(bg, fg, 5, -1)
	if len(strings.Split(res2, "\n")) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(strings.Split(res2, "\n")))
	}

	// Test 3: Extremely large positive posX & posY
	res3 := compositeOverlay(bg, fg, 100, 100)
	if len(strings.Split(res3, "\n")) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(strings.Split(res3, "\n")))
	}
}
