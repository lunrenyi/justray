package connection

// Watch registers a new status subscriber. Cancel deregisters it and, once
// the last watcher is gone, clears the probe cache.
func (s *Service) Watch() (initial Status, ch <-chan Status, cancel func()) {
	c := make(chan Status, 1)
	s.mu.Lock()
	s.watchers[c] = struct{}{}
	initial = s.status()
	s.mu.Unlock()

	return initial, c, func() {
		s.mu.Lock()
		delete(s.watchers, c)
		if len(s.watchers) == 0 {
			clear(s.probes)
		}
		s.mu.Unlock()
	}
}

func (s *Service) broadcast() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status()
	for ch := range s.watchers {
		select {
		case ch <- st:
		default:
		}
	}
	return st
}
