package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/luynrs/justray/internal/parser/protocols"
	"github.com/luynrs/justray/internal/parser/proxy"
)

var parsers = map[string]func(string) (proxy.Node, error){
	"vmess":     protocols.ParseVMess,
	"vless":     protocols.ParseVLess,
	"trojan":    protocols.ParseTrojan,
	"ss":        protocols.ParseSS,
	"hysteria":  protocols.ParseHY,
	"hysteria2": protocols.ParseHY2,
	"tuic":      protocols.ParseTUIC,
	"anytls":    protocols.ParseAnyTLS,
	"socks5":    protocols.ParseSOCKS,
}

func parserFor(uri string) func(string) (proxy.Node, error) {
	scheme, _, ok := strings.Cut(strings.TrimSpace(uri), "://")
	if !ok {
		return nil
	}
	switch scheme {
	case "hy2":
		scheme = "hysteria2"
	case "socks":
		scheme = "socks5"
	}
	return parsers[scheme]
}

func IsLink(s string) bool { return parserFor(s) != nil }

func ParseURI(uri string) (proxy.Node, error) {
	parse := parserFor(uri)
	if parse == nil {
		return proxy.Node{}, fmt.Errorf("unknown scheme in %.80q", uri)
	}
	return parse(strings.TrimSpace(uri))
}

// clash yaml (b64)
func ParseSubscription(raw []byte) ([]proxy.Node, error) {
	body := bytes.TrimSpace(raw)
	if nodes, err := protocols.ParseClash(body); err == nil {
		return nodes, nil
	}
	if decoded, err := protocols.Unbase64(string(body)); err == nil {
		body = decoded
		if nodes, err := protocols.ParseClash(body); err == nil {
			return nodes, nil
		}
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
