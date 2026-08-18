package daemon

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/parser"
	"github.com/luynrs/justray/internal/parser/proxy"
)

const maxBody = 10 << 20

func (s *Server) subs() ([]Sub, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, err := s.store.Subscriptions()
	if err != nil {
		return nil, err
	}
	out := make([]Sub, len(subs))
	for i, sub := range subs {
		out[i] = info(sub)
	}
	return out, nil
}

func (s *Server) addSub(rawURL string) (Sub, error) {
	if err := check(rawURL); err != nil {
		return Sub{}, err
	}

	// fetched without the lock
	sub := store.Subscription{ID: store.NewID(), URL: rawURL}
	if err := s.fill(&sub); err != nil {
		return Sub{}, err
	}
	if sub.Name == "" {
		sub.Name = host(rawURL)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	subs, err := s.store.Subscriptions()
	if err != nil {
		return Sub{}, err
	}
	return info(sub), s.store.Save(append(subs, sub))
}

func (s *Server) removeSub(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, err := s.store.Subscriptions()
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(subs, func(sub store.Subscription) bool { return sub.ID == id })
	if len(kept) == len(subs) {
		return fmt.Errorf("subscription %q not found", id)
	}
	if s.sub == id {
		s.clear()
	}
	return s.store.Save(kept)
}

func (s *Server) refreshAll() ([]Sub, error) {
	s.mu.Lock()
	subs, err := s.store.Subscriptions()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}

	out := make([]Sub, 0, len(subs))
	var failed error
	for i := range subs {
		if err := s.fill(&subs[i]); err != nil {
			failed = err
			s.log.Printf("refresh %s: %v", subs[i].Name, err)
			continue
		}
		out = append(out, info(subs[i]))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.Save(subs); err != nil {
		return nil, err
	}
	if failed != nil {
		return out, fmt.Errorf("%d of %d subscriptions failed, last: %w", len(subs)-len(out), len(subs), failed)
	}
	return out, nil
}

func (s *Server) refresh(id string) (Sub, error) {
	s.mu.Lock()
	subs, err := s.store.Subscriptions()
	s.mu.Unlock()
	if err != nil {
		return Sub{}, err
	}
	i := slices.IndexFunc(subs, func(sub store.Subscription) bool { return sub.ID == id })
	if i < 0 {
		return Sub{}, fmt.Errorf("subscription %q not found", id)
	}
	if err := s.fill(&subs[i]); err != nil {
		return Sub{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.Save(subs); err != nil {
		return Sub{}, err
	}
	return info(subs[i]), nil
}

func (s *Server) fill(sub *store.Subscription) error {
	if parser.IsLink(sub.URL) {
		n, err := parser.ParseURI(sub.URL)
		if err != nil {
			return err
		}
		sub.Nodes, sub.Name, sub.Traffic = []proxy.Node{n}, n.Name, store.Traffic{}
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

func (s *Server) fetch(rawURL string) ([]proxy.Node, string, store.Traffic, error) {
	var none store.Traffic

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", none, err
	}
	req.Header = s.device.Clone()

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, "", none, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode != http.StatusOK:
		return nil, "", none, fmt.Errorf("http %d", resp.StatusCode)
	case resp.Header.Get("X-Hwid-Max-Devices-Reached") == "true":
		return nil, "", none, fmt.Errorf("device limit reached")
	case resp.Header.Get("X-Hwid-Not-Supported") == "true":
		return nil, "", none, fmt.Errorf("this subscription requires a device id")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, "", none, err
	}
	nodes, err := parser.ParseSubscription(body)
	if err != nil {
		return nil, "", none, err
	}
	return nodes, title(resp.Header), usage(resp.Header), nil
}

func info(sub store.Subscription) Sub {
	return Sub{
		ID: sub.ID, Name: sub.Name, URL: sub.URL,
		Nodes: len(sub.Nodes), UpdatedAt: sub.UpdatedAt,
		Traffic: sub.Traffic, Direct: parser.IsLink(sub.URL),
	}
}

func check(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("paste a subscription url or a share link")
	}
	if parser.IsLink(rawURL) {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%q is not a url or a share link", rawURL)
	}
	return nil
}

func host(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return u.Host
	}
	return rawURL
}

// "Subscription-Userinfo: upload=N; download=N; total=N; expire=unixSeconds"
func usage(h http.Header) store.Traffic {
	var t store.Traffic
	for field := range strings.SplitSeq(h.Get("Subscription-Userinfo"), ";") {
		key, val, ok := strings.Cut(strings.TrimSpace(field), "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(key) {
		case "upload":
			t.UploadBytes = n
		case "download":
			t.DownloadBytes = n
		case "total":
			t.TotalBytes = n
		case "expire":
			if n > 0 { // 0 = never
				t.ExpiresAt = time.Unix(n, 0).UTC()
			}
		}
	}
	return t
}

func title(h http.Header) string {
	if t := h.Get("Profile-Title"); t != "" {
		t = strings.TrimPrefix(t, "base64:")
		if decoded, err := base64.StdEncoding.DecodeString(t); err == nil {
			return string(decoded)
		}
		if decoded, err := base64.RawURLEncoding.DecodeString(t); err == nil {
			return string(decoded)
		}
		return t
	}
	if cd := h.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			return params["filename"]
		}
	}
	return ""
}
