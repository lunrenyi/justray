package protocols

//
// Trojan
//

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"github.com/luynrs/justray/internal/parser/proxy"
)

func ParseTrojan(uri string) (proxy.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("trojan: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return proxy.Node{}, fmt.Errorf("trojan: missing password")
	}
	host, port, err := hostPort(u)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("trojan: %w", err)
	}

	q := u.Query()
	n := proxy.Node{
		ID:        id(uri),
		Name:      cmp.Or(u.Fragment, host),
		Protocol:  proxy.Trojan,
		Server:    host,
		Port:      port,
		Auth:      proxy.Auth{Password: u.User.Username()},
		Transport: transport(q),
	}
	if strings.ToLower(cmp.Or(q.Get("security"), "tls")) != "none" {
		n.TLS = &proxy.TLS{
			SNI:         cmp.Or(q.Get("sni"), q.Get("peer"), host),
			ALPN:        splitComma(q.Get("alpn")),
			Fingerprint: q.Get("fp"),
			Insecure:    truthy(q.Get("allowInsecure")),
		}
	}
	return n, nil
}
