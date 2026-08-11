package protocols

import (
	"cmp"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/luynrs/justxray/internal/parser/proxy"
)

func Unbase64(s string) ([]byte, error) {
	s = strings.Join(strings.Fields(s), "")
	s = strings.NewReplacer("-", "+", "_", "/").Replace(s)
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	return base64.StdEncoding.DecodeString(s)
}

func id(raw string) string {
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])[:12]
}

func hostPort(u *url.URL) (string, int, error) {
	host, p := u.Hostname(), u.Port()
	if host == "" || p == "" {
		return "", 0, fmt.Errorf("missing host or port")
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return "", 0, fmt.Errorf("bad port %q", p)
	}
	return host, port, nil
}

func transport(q url.Values) proxy.Transport {
	return proxy.Transport{
		Network:     strings.ToLower(cmp.Or(q.Get("type"), "tcp")),
		Path:        cmp.Or(q.Get("path"), q.Get("serviceName")),
		Host:        cmp.Or(q.Get("host"), q.Get("sni")),
		ServiceName: q.Get("serviceName"),
	}
}

func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// truthy covers insecure=1, allowInsecure=true and friends
func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// flexInt is a json num that some exporters quote as a str
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}
