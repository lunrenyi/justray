package protocols

//
// VLESS
//

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"github.com/luynrs/justxray/internal/parser/proxy"
)

// vless://uuid@host:port?...#remark
func ParseVLess(uri string) (proxy.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("vless: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return proxy.Node{}, fmt.Errorf("vless: missing uuid")
	}
	host, port, err := hostPort(u)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("vless: %w", err)
	}

	q := u.Query()
	n := proxy.Node{
		ID:       id(uri),
		Name:     cmp.Or(u.Fragment, host),
		Protocol: proxy.VLess,
		Server:   host,
		Port:     port,
		Auth:     proxy.Auth{UUID: u.User.Username(), Flow: q.Get("flow")},
		Transport: proxy.Transport{
			Network:     strings.ToLower(cmp.Or(q.Get("type"), "tcp")),
			Path:        cmp.Or(q.Get("path"), q.Get("serviceName")),
			Host:        cmp.Or(q.Get("host"), q.Get("sni")),
			ServiceName: q.Get("serviceName"),
		},
	}

	switch strings.ToLower(q.Get("security")) {
	case "reality":
		n.Reality = &proxy.Reality{PublicKey: q.Get("pbk"), ShortID: q.Get("sid"), SpiderX: q.Get("spx")}
		fallthrough
	case "tls":
		n.TLS = &proxy.TLS{
			SNI:         cmp.Or(q.Get("sni"), host),
			ALPN:        splitComma(q.Get("alpn")),
			Fingerprint: q.Get("fp"),
			Insecure:    truthy(q.Get("allowInsecure")),
		}
	}
	return n, nil
}
