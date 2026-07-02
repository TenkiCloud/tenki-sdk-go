package sandbox

import "time"

// pollBackoff returns the delay before the next poll attempt.
// First 3 attempts at 500ms (skip work that can't be done yet given typical
// RPC RTT and workflow start cost), then 5 tight attempts at 100ms to catch
// completion in the 1.5-2s window, then 1s steady for slower workflows.
func pollBackoff(attempt int) time.Duration {
	switch {
	case attempt < 3:
		return 500 * time.Millisecond
	case attempt < 8:
		return 100 * time.Millisecond
	default:
		return 1 * time.Second
	}
}
