package cli

import (
	"errors"
	"testing"
	"time"

	"github.com/luynrs/justray/internal/shared/rpc"
)

func TestAwaitElevate(t *testing.T) {
	elevatePoll = time.Millisecond
	tun := true

	replies := func(steps ...rpc.Status) func() (rpc.Status, error) {
		i := -1
		return func() (rpc.Status, error) {
			if i++; i >= len(steps) {
				return rpc.Status{}, errors.New("socket closed")
			}
			return steps[i], nil
		}
	}

	t.Run("waits out the restart", func(t *testing.T) {
		status := replies(
			rpc.Status{LastErr: rpc.ElevateMsg},     // old daemon, still on its way out
			rpc.Status{Connected: true, Tun: false}, // back up, tun not on yet
			rpc.Status{Connected: true, Tun: true},  // restored
		)
		st, err := awaitElevate(status, &tun, time.Second)
		if err != nil || !st.Tun {
			t.Fatalf("got %+v, %v; want the tun session, nil", st, err)
		}
	})

	t.Run("reports a real failure", func(t *testing.T) {
		status := replies(rpc.Status{LastErr: "operation not permitted"})
		if _, err := awaitElevate(status, &tun, time.Second); err == nil || err.Error() != "operation not permitted" {
			t.Fatalf("got %v; want the daemon's error", err)
		}
	})

	t.Run("times out", func(t *testing.T) {
		status := func() (rpc.Status, error) { return rpc.Status{}, errors.New("no daemon") }
		if _, err := awaitElevate(status, &tun, 10*time.Millisecond); err == nil {
			t.Fatal("want a timeout error")
		}
	})
}
