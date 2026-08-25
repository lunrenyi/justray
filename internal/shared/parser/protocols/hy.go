package protocols

// Hysteria v1

import (
	"cmp"

	"github.com/luynrs/justray/internal/shared/domain"
)

func ParseHY(uri string) (domain.Node, error) {
	u, host, port, err := parseURL("hysteria", uri)
	if err != nil {
		return domain.Node{}, err
	}

	q := u.Query()
	obfs := q.Get("obfsParam")
	if obfs == "" && q.Get("obfs") != "xplus" {
		obfs = q.Get("obfs")
	}
	n := domain.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: domain.HY1,
		Server:   host,
		Port:     port,
		Auth:     domain.Auth{Password: cmp.Or(q.Get("auth"), q.Get("auth_str"))},
		TLS: &domain.TLS{
			SNI:      cmp.Or(q.Get("peer"), q.Get("sni"), host),
			ALPN:     splitComma(q.Get("alpn")),
			Insecure: insecureFlag(q),
		},
		ObfsPassword: obfs,
		UpMbps:       cmp.Or(atoi(q.Get("upmbps")), 100),
		DownMbps:     cmp.Or(atoi(q.Get("downmbps")), 100),
	}
	return n, nil
}
