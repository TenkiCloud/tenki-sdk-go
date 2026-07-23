package sandbox

import (
	"strings"
)

const automaticPageSize int32 = 100

func advancePageToken(current, next string) (string, error) {
	next = strings.TrimSpace(next)
	if next != "" && next == current {
		return "", ErrPaginationStalled
	}
	return next, nil
}
