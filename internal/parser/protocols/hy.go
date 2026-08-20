package protocols

//
// Hysteria v1
//

import (
	"cmp"
	"fmt"
	"net/url"

	"github.com/luynrs/justray/internal/parser/proxy"
)

func ParseHY(uri string) (proxy.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("hysteria: %w", err)
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("hysteria: %w", err)
	}

	q := u.Query()
	obfs := q.Get("obfsParam")
	if obfs == "" && q.Get("obfs") != "xplus" { // xplus is the only type there is, so a bare obfs= is the password
		obfs = q.Get("obfs")
	}
	return proxy.Node{
		ID:       id(uri),
		Name:     cmp.Or(u.Fragment, host),
		Protocol: proxy.HY1,
		Server:   host,
		Port:     port,
		Auth:     proxy.Auth{Password: cmp.Or(q.Get("auth"), q.Get("auth_str"))},
		TLS: &proxy.TLS{
			SNI:      cmp.Or(q.Get("peer"), q.Get("sni"), host),
			ALPN:     splitComma(q.Get("alpn")),
			Insecure: truthy(cmp.Or(q.Get("insecure"), q.Get("allowInsecure"))),
		},
		ObfsPassword: obfs,
		UpMbps:       cmp.Or(atoi(q.Get("upmbps")), 100),   // brutal refuses to run on a zero rate
		DownMbps:     cmp.Or(atoi(q.Get("downmbps")), 100), // TODO: settings ui
	}, nil
}
