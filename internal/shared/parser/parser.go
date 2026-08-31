package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/luynrs/justray/internal/shared/domain"
	"github.com/luynrs/justray/internal/shared/parser/protocols"
)

var parsers = map[string]func(string) (domain.Node, error){
	"vmess":      protocols.ParseVMess,
	"vless":      protocols.ParseVLess,
	"trojan":     protocols.ParseTrojan,
	"ss":         protocols.ParseShadowsocks,
	"hysteria":   protocols.ParseHysteria,
	"hysteria2":  protocols.ParseHysteria2,
	"hy2":        protocols.ParseHysteria2,
	"tuic":       protocols.ParseTUIC,
	"anytls":     protocols.ParseAnyTLS,
	"socks5":     protocols.ParseSOCKS,
	"socks":      protocols.ParseSOCKS,
	"wireguard":  protocols.ParseWireGuard,
	"wg":         protocols.ParseWireGuard,
	"shadowtls":  protocols.ParseShadowTLS,
	"shadow-tls": protocols.ParseShadowTLS,
}

func parserFor(uri string) func(string) (domain.Node, error) {
	scheme, _, ok := strings.Cut(strings.TrimSpace(uri), "://")
	if !ok {
		return nil
	}
	return parsers[scheme]
}

func IsLink(s string) bool { return parserFor(s) != nil }

func ParseURI(uri string) (domain.Node, error) {
	parse := parserFor(uri)
	if parse == nil {
		return domain.Node{}, fmt.Errorf("unknown scheme in %.80q", uri)
	}
	n, err := parse(strings.TrimSpace(uri))
	if err != nil {
		return domain.Node{}, err
	}
	n.ID = protocols.NodeID(n)
	return n, nil
}

// clash yaml (b64)
func ParseSubscription(raw []byte) ([]domain.Node, error) {
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

	var nodes []domain.Node
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
			return nil, bad
		}
		return nil, fmt.Errorf("no nodes in subscription")
	}
	return nodes, nil
}
