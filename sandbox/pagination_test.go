package sandbox

import (
	"errors"
	"testing"
)

func TestAdvancePageToken(t *testing.T) {
	t.Parallel()

	next, err := advancePageToken("", " next ")
	if err != nil || next != "next" {
		t.Fatalf("advancePageToken = %q, %v", next, err)
	}
	if _, err := advancePageToken("next", " next "); !errors.Is(err, ErrPaginationStalled) {
		t.Fatalf("advancePageToken error = %v, want ErrPaginationStalled", err)
	}
}
