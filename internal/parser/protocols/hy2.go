package protocols

//
// Hysteria v2
//

import (
	"cmp"
	"fmt"
	"net/url"

	"github.com/luynrs/justray/internal/parser/proxy"
)

func ParseHY2(uri string) (proxy.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("hysteria2: %w", err)
	}
	auth := ""
	if u.User != nil {
		auth = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			auth += ":" + pw
		}
	}
	if auth == "" {
		return proxy.Node{}, fmt.Errorf("hysteria2: missing auth")
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("hysteria2: %w", err)
	}

	q := u.Query()
	n := proxy.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: proxy.HY2,
		Server:   host,
		Port:     port,
		Auth:     proxy.Auth{Password: auth},
		TLS: &proxy.TLS{
			SNI:      cmp.Or(q.Get("sni"), q.Get("peer"), host),
			Insecure: insecureFlag(q),
		},
		Obfs:         q.Get("obfs"),
		ObfsPassword: q.Get("obfs-password"),
	}
	n.ID = nodeID(n)
	return n, nil
}
