package outbound

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/luynrs/justray/internal/domain"
)

func transport(n domain.Node) *option.V2RayTransportOptions {
	switch n.Transport.Network {
	case "ws":
		ws := option.V2RayWebsocketOptions{Path: n.Transport.Path}
		if n.Transport.Host != "" {
			ws.Headers = badoption.HTTPHeader{"Host": {n.Transport.Host}}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: ws}
	case "grpc":
		return &option.V2RayTransportOptions{
			Type:        C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{ServiceName: n.Transport.ServiceName},
		}
	case "http":
		h := option.V2RayHTTPOptions{Path: n.Transport.Path}
		if n.Transport.Host != "" {
			h.Host = badoption.Listable[string]{n.Transport.Host}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeHTTP, HTTPOptions: h}
	case "httpupgrade":
		return &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeHTTPUpgrade,
			HTTPUpgradeOptions: option.V2RayHTTPUpgradeOptions{
				Path: n.Transport.Path,
				Host: n.Transport.Host,
			},
		}
	case "xhttp", "splithttp":
		return &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeXHTTP,
			XHTTPOptions: option.V2RayXHTTPOptions{
				Path: n.Transport.Path,
				Host: n.Transport.Host,
				Mode: n.Transport.Mode,
			},
		}
	}
	return nil
}
