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
	if zeroPad.XPaddingBytes != "0-0" {
		t.Fatalf("expected 0-0 XPaddingBytes, got %q", zeroPad.XPaddingBytes)
	}

	snake := xhttpOptions(domain.Transport{Extra: `{"mode":"packet-up","session_placement":"header","session_key":"hdr_sid","x_padding_bytes":"200-400","headers":{"Custom":"val"}}`})
	if snake.Mode != "packet-up" || snake.SessionPlacement != "header" || snake.SessionKey != "hdr_sid" || snake.XPaddingBytes != "200-400" || len(snake.Headers["Custom"]) == 0 || snake.Headers["Custom"][0] != "val" {
		t.Fatalf("unexpected snake options: %+v", snake)
	}
}
