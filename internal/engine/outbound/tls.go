package outbound

import (
	"github.com/sagernet/sing-box/option"

	"github.com/luynrs/justray/internal/domain"
)

func tlsOptions(n domain.Node) *option.OutboundTLSOptions {
	switch {
	case n.Reality != nil:
		tls := &option.OutboundTLSOptions{
			Enabled: true,
			Reality: &option.OutboundRealityOptions{
				Enabled:   true,
				PublicKey: n.Reality.PublicKey,
				ShortID:   n.Reality.ShortID,
			},
			UTLS: &option.OutboundUTLSOptions{Enabled: true, Fingerprint: "chrome"},
		}
		if n.TLS != nil {
			tls.ServerName = n.TLS.SNI
			tls.Insecure = n.TLS.Insecure
			tls.ALPN = n.TLS.ALPN
			if n.TLS.Fingerprint != "" {
				tls.UTLS.Fingerprint = n.TLS.Fingerprint
			}
		}
		return tls

	case n.TLS != nil:
		tls := &option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: n.TLS.SNI,
			Insecure:   n.TLS.Insecure,
			ALPN:       n.TLS.ALPN,
		}
		if n.TLS.Fingerprint != "" {
			tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: n.TLS.Fingerprint}
		}
		return tls
	}
	return nil
}
