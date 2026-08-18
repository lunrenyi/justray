package daemon

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/luynrs/justxray/internal/daemon/procgroup"
	"github.com/luynrs/justxray/internal/daemon/xray"
	"github.com/luynrs/justxray/internal/parser/proxy"
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
		if _, err := xray.Outbound(n, "p"); err == nil {
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
	stop, err := s.startProbeXray(testable, ports)
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

func (s *Server) startProbeXray(nodes []proxy.Node, ports []int) (func(), error) {
	cfg, err := xray.ProbeConfig(nodes, ports)
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(s.dir, "probe-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(cfg); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()

	cmd := exec.Command(s.xrayBin, "run", "-c", f.Name())
	procgroup.Setup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	stop := func() {
		procgroup.Kill(cmd)
		cmd.Wait()
	}

	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		if conn, err := net.DialTimeout("tcp", local(ports[0]), 200*time.Millisecond); err == nil {
			conn.Close()
			return stop, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	return nil, fmt.Errorf("xray did not come up for the latency test")
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
