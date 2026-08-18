package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	sbox "github.com/sagernet/sing-box"

	"github.com/luynrs/justray/internal/daemon/core"
	"github.com/luynrs/justray/internal/parser/proxy"
)

const (
	probeURL     = "http://cp.cloudflare.com/generate_204"
	probeTimeout = 5 * time.Second
	probeWorkers = 12
)

type probeResult struct {
	alive bool
	ms    int
}

func (s *Server) probeNodes(nodes []proxy.Node) (map[string]probeResult, error) {
	var testable []proxy.Node
	for _, n := range nodes {
		if _, err := core.Outbound(n, "p"); err == nil {
			testable = append(testable, n)
		}
	}
	out := map[string]probeResult{}
	if len(testable) == 0 {
		return out, nil
	}

	ports, err := freePorts(len(testable))
	if err != nil {
		return nil, err
	}
	stop, err := s.startProbeCore(testable, ports)
	if err != nil {
		return nil, err
	}
	defer stop()

	sem := make(chan struct{}, probeWorkers)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, n := range testable {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			ms, err := delay(ports[i])
			mu.Lock()
			out[n.ID] = probeResult{alive: err == nil, ms: ms}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out, nil
}

func (s *Server) startProbeCore(nodes []proxy.Node, ports []int) (func(), error) {
	opts, err := core.ProbeConfig(nodes, ports)
	if err != nil {
		return nil, err
	}
	inst, err := sbox.New(sbox.Options{Options: *opts, Context: core.Context(context.Background())})
	if err != nil {
		return nil, fmt.Errorf("build probe engine: %w", err)
	}
	if err := inst.Start(); err != nil {
		inst.Close()
		return nil, fmt.Errorf("start probe engine: %w", err)
	}
	return func() { inst.Close() }, nil
}

func delay(port int) (int, error) {
	client := &http.Client{
		Timeout:   probeTimeout,
		Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "socks5", Host: local(port)})},
	}
	start := time.Now()
	resp, err := client.Get(probeURL)
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		return ms, err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ms, fmt.Errorf("http %d", resp.StatusCode)
	}
	return ms, nil
}

func freePorts(n int) ([]int, error) {
	ports := make([]int, n)
	for i := range ports {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		defer ln.Close()
		ports[i] = ln.Addr().(*net.TCPAddr).Port
	}
	return ports, nil
}

func local(port int) string { return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) }
