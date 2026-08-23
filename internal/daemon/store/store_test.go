package store

import (
	"reflect"
	"testing"
	"time"

	"github.com/luynrs/justray/internal/shared/domain"
)

func TestStore(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		d := Disk{Dir: t.TempDir()}

		if subs, err := d.Subscriptions(); err != nil || subs != nil {
			t.Fatalf("empty dir: got %v, %v; want nil, nil", subs, err)
		}

		subs := []Subscription{{
			ID: "a", Name: "test", URL: "https://example.com/sub",
			UpdatedAt: time.Now().Truncate(time.Second).UTC(),
			Traffic:   domain.Traffic{UploadBytes: 1, DownloadBytes: 2, TotalBytes: 3},
			Nodes: []domain.Node{{
				ID: "n1", Name: "node", Protocol: domain.VLess,
				Server: "1.2.3.4", Port: 443, Auth: domain.Auth{UUID: "uuid"},
			}},
		}}
		if err := d.Save(subs); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := d.Subscriptions()
		if err != nil {
			t.Fatalf("Subscriptions: %v", err)
		}
		if !reflect.DeepEqual(got, subs) {
			t.Fatalf("got %+v, want %+v", got, subs)
		}
	})

	t.Run("active", func(t *testing.T) {
		d := Disk{Dir: t.TempDir()}

		if id, err := d.Active(); err != nil || id != "" {
			t.Fatalf("empty dir: got %q, %v; want \"\", nil", id, err)
		}
		if err := d.SetActive("node1"); err != nil {
			t.Fatalf("SetActive: %v", err)
		}
		if id, err := d.Active(); err != nil || id != "node1" {
			t.Fatalf("got %q, %v; want \"node1\", nil", id, err)
		}
	})

	t.Run("tun", func(t *testing.T) {
		d := Disk{Dir: t.TempDir()}

		if on, err := tun(d); err != nil || on {
			t.Fatalf("empty dir: got %v, %v; want false, nil", on, err)
		}
		if err := d.SetTun(true); err != nil {
			t.Fatalf("SetTun(true): %v", err)
		}
		if on, err := tun(d); err != nil || !on {
			t.Fatalf("got %v, %v; want true, nil", on, err)
		}
		if err := d.SetTun(false); err != nil {
			t.Fatalf("SetTun(false): %v", err)
		}
		if on, err := tun(d); err != nil || on {
			t.Fatalf("got %v, %v; want false, nil", on, err)
		}
	})
}

func TestNewID(t *testing.T) {
	a, b := NewID(), NewID()
	if a == b {
		t.Fatalf("got same id %q", a)
	}
	if len(a) != 8 {
		t.Fatalf("len(%q) = %d, want 8", a, len(a))
	}
}

func tun(d Disk) (bool, error) {
	s, err := d.State()
	return s.Tun, err
}
