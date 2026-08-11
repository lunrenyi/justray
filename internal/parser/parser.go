package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/luynrs/justxray/internal/parser/protocols"
	"github.com/luynrs/justxray/internal/parser/proxy"
)

var parsers = map[string]func(string) (proxy.Node, error){
	"vmess":     protocols.ParseVMess,
	"vless":     protocols.ParseVLess,
	"trojan":    protocols.ParseTrojan,
	"ss":        protocols.ParseSS,
	"hysteria2": protocols.ParseHY2,
	"hy2":       protocols.ParseHY2,
}

func IsLink(s string) bool {
	scheme, _, ok := strings.Cut(strings.TrimSpace(s), "://")
	_, known := parsers[scheme]
	return ok && known
}

func ParseURI(uri string) (proxy.Node, error) {
	uri = strings.TrimSpace(uri)
	scheme, _, _ := strings.Cut(uri, "://")
	parse, ok := parsers[scheme]
	if !ok {
		return proxy.Node{}, fmt.Errorf("unknown scheme in %q", uri)
	}
	return parse(uri)
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
