package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/luynrs/justxray/internal/parser/protocols"
	"github.com/luynrs/justxray/internal/parser/proxy"
)

var schemes = []string{"vmess", "vless", "trojan", "ss", "hysteria2", "hy2"}

func IsLink(s string) bool {
	scheme, _, ok := strings.Cut(strings.TrimSpace(s), "://")
	if !ok {
		return false
	}
	for _, k := range schemes {
		if scheme == k {
			return true
		}
	}
	return false
}

func ParseURI(uri string) (proxy.Node, error) {
	uri = strings.TrimSpace(uri)
	scheme, _, _ := strings.Cut(uri, "://")
	switch scheme {
	case "vmess":
		return protocols.ParseVMess(uri)
	case "vless":
		return protocols.ParseVLess(uri)
	case "trojan":
		return protocols.ParseTrojan(uri)
	case "ss":
		return protocols.ParseSS(uri)
	case "hysteria2", "hy2":
		return protocols.ParseHY2(uri)
	}
	return proxy.Node{}, fmt.Errorf("unknown scheme in %q", uri)
}

// clash yaml (b64)
func ParseSubscription(raw []byte) ([]proxy.Node, error) {
	body := bytes.TrimSpace(raw)
	if nodes, err := protocols.ParseClash(body); err == nil {
		return nodes, nil
	}
	if decoded, err := protocols.Unbase64(string(body)); err == nil {
		body = decoded
	}

	var nodes []proxy.Node
	var bad error
	for line := range strings.Lines(string(body)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		n, err := ParseURI(line)
		if err != nil {
			bad = err
			continue
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		if bad != nil {
			return nil, fmt.Errorf("no valid nodes, last error: %w", bad)
		}
		return nil, fmt.Errorf("no nodes in subscription")
	}
	return nodes, nil
}
