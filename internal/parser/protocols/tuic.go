package protocols

//
// TUIC v5
//

import (
	"cmp"
	"fmt"
	"net/url"

	"github.com/luynrs/justray/internal/parser/proxy"
)

func ParseTUIC(uri string) (proxy.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("tuic: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return proxy.Node{}, fmt.Errorf("tuic: missing uuid")
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("tuic: %w", err)
	}

	q := u.Query()
	password, _ := u.User.Password()
	n := proxy.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: proxy.TUIC,
		Server:   host,
		Port:     port,
		Auth:     proxy.Auth{UUID: u.User.Username(), Password: cmp.Or(password, q.Get("password"))},
		TLS: &proxy.TLS{
			SNI:      cmp.Or(q.Get("sni"), host),
			ALPN:     splitComma(cmp.Or(q.Get("alpn"), "h3")),
			Insecure: insecureFlag(q),
		},
		Congestion:   cmp.Or(q.Get("congestion_control"), "bbr"),
		UDPRelayMode: cmp.Or(q.Get("udp_relay_mode"), "native"),
	}
	n.ID = nodeID(n)
	return n, nil
}
