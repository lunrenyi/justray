package protocols

//
// AnyTLS
//

import (
	"cmp"
	"fmt"
	"net/url"

	"github.com/luynrs/justray/internal/parser/proxy"
)

func ParseAnyTLS(uri string) (proxy.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("anytls: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return proxy.Node{}, fmt.Errorf("anytls: missing password")
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("anytls: %w", err)
	}

	q := u.Query()
	return proxy.Node{
		ID:       id(uri),
		Name:     cmp.Or(u.Fragment, host),
		Protocol: proxy.AnyTLS,
		Server:   host,
		Port:     port,
		Auth:     proxy.Auth{Password: u.User.Username()},
		TLS: &proxy.TLS{
			SNI:         cmp.Or(q.Get("sni"), q.Get("peer"), host),
			ALPN:        splitComma(q.Get("alpn")),
			Fingerprint: q.Get("fp"),
			Insecure:    truthy(cmp.Or(q.Get("insecure"), q.Get("allowInsecure"))),
		},
	}, nil
}
