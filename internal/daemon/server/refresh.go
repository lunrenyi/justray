package server

import "time"

func (s *Server) AutoRefresh(done <-chan struct{}) {
	tried := map[string]time.Time{}
	for {
		select {
		case <-done:
			return
		case <-time.After(time.Minute):
		}

		every := time.Duration(s.conn.RefreshEvery()) * time.Hour
		if every == 0 {
			continue
		}
		for _, id := range s.stale(every) {
			if time.Since(tried[id]) < 15*time.Minute {
				continue
			}
			tried[id] = time.Now()
			if _, err := s.subs.Refresh(id); err != nil {
				s.log.Print(err)
			}
		}
	}
}

func (s *Server) stale(every time.Duration) []string {
	list, err := s.subs.List()
	if err != nil {
		s.log.Printf("auto refresh: %v", err)
		return nil
	}
	var out []string
	for _, sub := range list {
		if time.Since(sub.UpdatedAt) >= every {
			out = append(out, sub.ID)
		}
	}
	return out
}
