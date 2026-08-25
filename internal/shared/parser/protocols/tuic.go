package protocols

// TUIC v5

import (
	"cmp"
	"fmt"

	"github.com/luynrs/justray/internal/shared/domain"
)

func ParseTUIC(uri string) (domain.Node, error) {
	u, host, port, err := parseURL("tuic", uri)
	if err != nil {
		return domain.Node{}, err
	}
	if u.User == nil || u.User.Username() == "" {
		return domain.Node{}, fmt.Errorf("tuic: missing uuid")
	}

	q := u.Query()
	password, _ := u.User.Password()
	n := domain.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: domain.TUIC,
		Server:   host,
		Port:     port,
		Auth:     domain.Auth{UUID: u.User.Username(), Password: cmp.Or(password, q.Get("password"))},
		TLS: &domain.TLS{
			SNI:      cmp.Or(q.Get("sni"), host),
			ALPN:     splitComma(cmp.Or(q.Get("alpn"), "h3")),
			Insecure: insecureFlag(q),
		},
		Congestion:   cmp.Or(q.Get("congestion_control"), "bbr"),
		UDPRelayMode: cmp.Or(q.Get("udp_relay_mode"), "native"),
	}
	return n, nil
}
