package protocols

// SOCKS5

import (
	"cmp"
	"fmt"
	"net/url"

	"github.com/luynrs/justray/internal/shared/domain"
)

func ParseSOCKS(uri string) (domain.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return domain.Node{}, fmt.Errorf("socks: %w", err)
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return domain.Node{}, fmt.Errorf("socks: %w", err)
	}

	var user, password string
	if u.User != nil {
		user = u.User.Username()
		if p, ok := u.User.Password(); ok {
			password = p
		} else {
			user, password = splitCreds(user)
		}
	}
	n := domain.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: domain.SOCKS,
		Server:   host,
		Port:     port,
		Auth:     domain.Auth{Username: user, Password: password},
	}
	n.ID = nodeID(n)
	return n, nil
}
