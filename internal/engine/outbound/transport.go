package outbound

import (
	"cmp"
	"encoding/json"
	"strings"

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
			Type:         C.V2RayTransportTypeXHTTP,
			XHTTPOptions: xhttpOptions(n.Transport),
		}
	}
	return nil
}

type xhttpExtra struct {
	Path                 string `json:"path"`
	Host                 string `json:"host"`
	Mode                 string `json:"mode"`
	NoGRPCHeader         bool   `json:"noGRPCHeader"`
	SessionIDPlacement   string `json:"sessionIDPlacement"`
	SessionIDKey         string `json:"sessionIDKey"`
	SessionPlacement     string `json:"sessionPlacement"`
	SessionKey           string `json:"sessionKey"`
	SessionTable         string `json:"sessionTable"`
	SessionLength        string `json:"sessionLength"`
	SeqPlacement         string `json:"seqPlacement"`
	SeqKey               string `json:"seqKey"`
	UplinkDataPlacement  string `json:"uplinkDataPlacement"`
	UplinkDataKey        string `json:"uplinkDataKey"`
	UplinkChunkSize      string `json:"uplinkChunkSize"`
	UplinkHTTPMethod     string `json:"uplinkHTTPMethod"`
	XPaddingBytes        string `json:"xPaddingBytes"`
	XPaddingObfsMode     bool   `json:"xPaddingObfsMode"`
	XPaddingKey          string `json:"xPaddingKey"`
	XPaddingHeader       string `json:"xPaddingHeader"`
	XPaddingPlacement    string `json:"xPaddingPlacement"`
	XPaddingMethod       string `json:"xPaddingMethod"`
	ScMaxEachPostBytes   string `json:"scMaxEachPostBytes"`
	ScMinPostsIntervalMs string `json:"scMinPostsIntervalMs"`
}

func xhttpOptions(t domain.Transport) option.V2RayXHTTPOptions {
	opts := option.V2RayXHTTPOptions{Path: t.Path, Host: t.Host, Mode: cmp.Or(t.Mode, "auto")}
	if t.Extra != "" {
		var e xhttpExtra
		if json.Unmarshal([]byte(t.Extra), &e) == nil {
			opts.Path = cmp.Or(opts.Path, e.Path)
			opts.Host = cmp.Or(opts.Host, e.Host)
			opts.Mode = cmp.Or(opts.Mode, e.Mode, "auto")
			opts.NoGRPCHeader = e.NoGRPCHeader
			opts.SessionPlacement = cmp.Or(e.SessionIDPlacement, e.SessionPlacement)
			opts.SessionKey = cmp.Or(e.SessionIDKey, e.SessionKey)
			opts.SessionTable = e.SessionTable
			opts.SessionLength = e.SessionLength
			opts.SeqPlacement = e.SeqPlacement
			opts.SeqKey = e.SeqKey
			opts.UplinkDataPlacement = e.UplinkDataPlacement
			opts.UplinkDataKey = e.UplinkDataKey
			opts.UplinkChunkSize = e.UplinkChunkSize
			opts.UplinkHTTPMethod = e.UplinkHTTPMethod
			if e.XPaddingBytes != "" && e.XPaddingBytes != "0-0" && e.XPaddingBytes != "0" {
				opts.XPaddingBytes = e.XPaddingBytes
			}
			opts.XPaddingObfsMode = e.XPaddingObfsMode
			opts.XPaddingKey = e.XPaddingKey
			opts.XPaddingHeader = e.XPaddingHeader
			opts.XPaddingPlacement = e.XPaddingPlacement
			opts.XPaddingMethod = e.XPaddingMethod
			opts.ScMaxEachPostBytes = e.ScMaxEachPostBytes
			opts.ScMinPostsIntervalMs = e.ScMinPostsIntervalMs
		}
	}
	if strings.EqualFold(opts.UplinkHTTPMethod, "GET") && (opts.Mode == "" || opts.Mode == "auto") {
		opts.Mode = "packet-up"
	}
	return opts
}
