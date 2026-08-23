package protocols

// Trojan

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"github.com/luynrs/justray/internal/shared/domain"
)

func ParseTrojan(uri string) (domain.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return domain.Node{}, fmt.Errorf("trojan: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return domain.Node{}, fmt.Errorf("trojan: missing password")
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return domain.Node{}, fmt.Errorf("trojan: %w", err)
	}

	q := u.Query()
	n := domain.Node{
		Name:      cmp.Or(u.Fragment, host),
		Protocol:  domain.Trojan,
		Server:    host,
		Port:      port,
		Auth:      domain.Auth{Password: u.User.Username()},
		Transport: transport(q),
	}
	if strings.ToLower(cmp.Or(q.Get("security"), "tls")) != "none" {
		n.TLS = &domain.TLS{
			SNI:         cmp.Or(q.Get("sni"), q.Get("peer"), host),
			ALPN:        splitComma(q.Get("alpn")),
			Fingerprint: q.Get("fp"),
			Insecure:    insecureFlag(q),
		}
	}
	n.ID = nodeID(n)
	return n, nil
}
