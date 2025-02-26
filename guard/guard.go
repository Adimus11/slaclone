package guard

import "sync"

type ChanGuard[T any] struct {
	mu     sync.Mutex
	c      chan T
	closed bool
}

func NewSafeChan[T any]() *ChanGuard[T] {
	return &ChanGuard[T]{
		c: make(chan T),
	}
}

func (s *ChanGuard[T]) Send(event T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.c <- event
}

func (s *ChanGuard[T]) Receiver() <-chan T {
	return s.c
}

func (s *ChanGuard[T]) Close() {
	go func() {
		for _, ok := <-s.c; ok; {
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.c)
	s.closed = true
}
