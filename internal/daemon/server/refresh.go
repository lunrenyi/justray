package server

import "time"

const refreshTick = time.Minute

// AutoRefresh refetches subscriptions whose UpdatedAt aged out
func (s *Server) AutoRefresh(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-time.After(refreshTick):
		}

		every := time.Duration(s.conn.RefreshEvery()) * time.Hour
		if every == 0 || !s.stale(every) {
			continue
		}
		if _, err := s.subs.RefreshAll(); err != nil {
			s.log.Printf("auto refresh: %v", err)
		}
	}
}

func (s *Server) stale(every time.Duration) bool {
	list, err := s.subs.List()
	if err != nil {
		s.log.Printf("auto refresh: %v", err)
		return false
	}
	for _, sub := range list {
		if time.Since(sub.UpdatedAt) >= every {
			return true
		}
	}
	return false
}
