package outbound

import (
	"testing"

	"github.com/luynrs/justray/internal/domain"
)

func TestXHTTPOptions(t *testing.T) {
	got := xhttpOptions(domain.Transport{Extra: `{"path":"/xhttp","sessionIDPlacement":"query","sessionIDKey":"sid","seqKey":"seq","uplinkDataKey":"data","uplinkHTTPMethod":"GET","xPaddingObfsMode":true,"xPaddingKey":"pad","xPaddingMethod":"tokenish"}`})

	if got.Path != "/xhttp" || got.Mode != "packet-up" || got.SessionPlacement != "query" || got.SessionKey != "sid" || got.SeqKey != "seq" || got.UplinkDataKey != "data" || got.UplinkHTTPMethod != "GET" || !got.XPaddingObfsMode || got.XPaddingKey != "pad" || got.XPaddingMethod != "tokenish" {
		t.Fatalf("unexpected options: %+v", got)
	}

	zeroPad := xhttpOptions(domain.Transport{Extra: `{"xPaddingBytes":"0-0"}`})
	if zeroPad.XPaddingBytes != "" {
		t.Fatalf("expected empty XPaddingBytes for 0-0, got %q", zeroPad.XPaddingBytes)
	}
}

