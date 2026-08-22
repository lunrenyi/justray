package protocols

//
// AnyTLS
//

import (
	"cmp"
	"fmt"
	"net/url"

	"github.com/luynrs/justray/internal/shared/domain"
)

func ParseAnyTLS(uri string) (domain.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return domain.Node{}, fmt.Errorf("anytls: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return domain.Node{}, fmt.Errorf("anytls: missing password")
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return domain.Node{}, fmt.Errorf("anytls: %w", err)
	}

	q := u.Query()
	n := domain.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: domain.AnyTLS,
		Server:   host,
		Port:     port,
		Auth:     domain.Auth{Password: u.User.Username()},
		TLS: &domain.TLS{
			SNI:         cmp.Or(q.Get("sni"), q.Get("peer"), host),
			ALPN:        splitComma(q.Get("alpn")),
			Fingerprint: q.Get("fp"),
			Insecure:    insecureFlag(q),
		},
	}
	n.ID = nodeID(n)
	return n, nil
}
