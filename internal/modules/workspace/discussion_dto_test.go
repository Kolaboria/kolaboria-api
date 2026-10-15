package workspace

import (
	"strings"
	"testing"
)

func TestNormalizeDiscussionTitleTrimsAndCountsUnicodeRunes(t *testing.T) {
	value, err := normalizeDiscussionTitle("  Halo 世界  ")
	if err != nil {
		t.Fatalf("normalize title: %v", err)
	}
	if value != "Halo 世界" {
		t.Fatalf("expected trimmed title, got %q", value)
	}

	tooLong := strings.Repeat("界", DiscussionTitleMaxLength+1)
	if _, err := normalizeDiscussionTitle(tooLong); err == nil {
		t.Fatal("expected title over Unicode rune limit to be rejected")
	}
}

func TestNormalizeDiscussionBodyRejectsWhitespaceAndOverLimit(t *testing.T) {
	if _, err := normalizeDiscussionBody(" \n\t "); err == nil {
		t.Fatal("expected blank body to be rejected")
	}
	if _, err := normalizeDiscussionBody(strings.Repeat("a", DiscussionBodyMaxLength+1)); err == nil {
		t.Fatal("expected body over limit to be rejected")
	}
	if _, err := normalizeDiscussionReplyBody(strings.Repeat("界", DiscussionReplyMaxLength+1)); err == nil {
		t.Fatal("expected reply over Unicode rune limit to be rejected")
	}
}
