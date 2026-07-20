package sandbox

import (
	"io"
	"sync"
)

const streamBufferSize = 16

// Stream yields incremental command output and a separate terminal result.
// Chunks are delivered through Next; terminal command status comes from Wait.
type Stream struct {
	cancel func()

	chunks      chan Output
	drainOnce   sync.Once
	drainCh     chan struct{}
	resultReady chan struct{}

	mu     sync.Mutex
	result *Result
	err    error
}

// Next returns the next stdout/stderr chunk. It returns io.EOF once the stream
// has been drained. If Wait already drained unread chunks, Next may reach EOF
// earlier than the remote command's natural output boundary.
func (s *Stream) Next() (Output, error) {
	output, ok := <-s.chunks
	if ok {
		return output, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return Output{}, s.err
	}
	return Output{}, io.EOF
}

// Wait blocks until the command reaches a terminal state. It may drain and
// discard unread chunks so callers do not need to fully consume Next to get the
// final result. Multiple Wait calls return the same cached result.
func (s *Stream) Wait() (*Result, error) {
	s.requestDrain()
	<-s.resultReady
	s.discardBufferedChunks()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

// Cancel stops local stream consumption. It does not guarantee remote command
// termination and Wait will subsequently return the local cancellation error.
func (s *Stream) Cancel() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
}

func (s *Stream) emit(output Output) bool {
	select {
	case <-s.drainCh:
		return false
	default:
	}

	select {
	case s.chunks <- output:
		return true
	case <-s.drainCh:
		return false
	}
}

func (s *Stream) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func (s *Stream) requestDrain() {
	s.drainOnce.Do(func() {
		close(s.drainCh)
	})
}

func (s *Stream) discardBufferedChunks() {
	for {
		select {
		case _, ok := <-s.chunks:
			if !ok {
				return
			}
		default:
			return
		}
	}
}
