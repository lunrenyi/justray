package protocols

// Hysteria v2

import (
	"cmp"
	"fmt"

	"github.com/luynrs/justray/internal/domain"
)

func ParseHysteria2(uri string) (domain.Node, error) {
	u, host, port, err := parseURL("hysteria2", uri)
	if err != nil {
		return domain.Node{}, err
	}
	auth := ""
	if u.User != nil {
		auth = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			auth += ":" + pw
		}
	}
	if auth == "" {
		return domain.Node{}, fmt.Errorf("hysteria2: missing auth")
	}

	q := u.Query()
	n := domain.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: domain.HY2,
		Server:   host,
		Port:     port,
		Auth:     domain.Auth{Password: auth},
		TLS: &domain.TLS{
			SNI:      cmp.Or(q.Get("sni"), q.Get("peer"), host),
			Insecure: insecureFlag(q),
		},
		Obfs:         q.Get("obfs"),
		ObfsPassword: q.Get("obfs-password"),
	}
	return n, nil
}
