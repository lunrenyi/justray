package subscription

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/shared/domain"
	"github.com/luynrs/justray/internal/shared/parser"
	"github.com/luynrs/justray/internal/shared/rpc"
)

func (s *Service) RefreshAll() ([]rpc.Sub, error) {
	subs, err := s.store.Subscriptions()
	if err != nil {
		return nil, err
	}

	errs := make([]error, len(subs))
	var wg sync.WaitGroup
	for i := range subs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.fill(&subs[i])
		}()
	}
	wg.Wait()

	out := make([]rpc.Sub, len(subs))
	updated := make([]store.Subscription, 0, len(subs))
	var failed error
	for i, err := range errs {
		// on failure subs[i] keeps its pre-refresh data
		out[i] = info(subs[i])
		if err != nil {
			failed = err
			s.log.Print(err)
			continue
		}
		updated = append(updated, subs[i])
	}

	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	if err := s.merge(updated); err != nil {
		return nil, err
	}
	if failed != nil {
		return out, fmt.Errorf("%d of %d subscriptions failed, last: %w", len(subs)-len(updated), len(subs), failed)
	}
	return out, nil
}

func (s *Service) Refresh(id string) (rpc.Sub, error) {
	subs, err := s.store.Subscriptions()
	if err != nil {
		return rpc.Sub{}, err
	}
	i := slices.IndexFunc(subs, func(sub store.Subscription) bool { return sub.ID == id })
	if i < 0 {
		return rpc.Sub{}, fmt.Errorf("subscription %q not found", id)
	}
	if err := s.fill(&subs[i]); err != nil {
		return rpc.Sub{}, err
	}

	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	if err := s.merge(subs[i : i+1]); err != nil {
		return rpc.Sub{}, err
	}
	return info(subs[i]), nil
}

func (s *Service) merge(updated []store.Subscription) error {
	subs, err := s.store.Subscriptions()
	if err != nil {
		return err
	}
	for _, u := range updated {
		if i := slices.IndexFunc(subs, func(x store.Subscription) bool { return x.ID == u.ID }); i >= 0 {
			subs[i] = u
		}
	}
	return s.store.Save(subs)
}

func (s *Service) fill(sub *store.Subscription) error {
	if parser.IsLink(sub.URL) {
		n, err := parser.ParseURI(sub.URL)
		if err != nil {
			return err
		}
		sub.Nodes, sub.Name, sub.Traffic = []domain.Node{n}, n.Name, domain.Traffic{}
		sub.UpdatedAt = time.Now().UTC()
		return nil
	}

	nodes, name, traffic, err := s.fetch(sub.URL)
	if err != nil {
		return err
	}
	sub.Nodes, sub.Traffic, sub.UpdatedAt = nodes, traffic, time.Now().UTC()
	if name != "" { // change name if it changed on server
		sub.Name = name
	}
	return nil
}
