package cli

import "testing"

func TestMatch(t *testing.T) {
	type item struct{ id, name string }
	items := []item{
		{"74df0a1f", "Frankfurt am Main"},
		{"9e96741d", "Frankfurt am Main gRPC"},
		{"7e022a3a", "Prague"},
	}
	idName := func(i item) (string, string) { return i.id, i.name }

	for _, tc := range []struct {
		key, want string
	}{
		{"7e022a3a", "Prague"},
		{"7e02", "Prague"},
		{"prague", "Prague"},
		{"PRAGUE", "Prague"},
		{"grpc", "Frankfurt am Main gRPC"},
	} {
		got, err := match(tc.key, "node", items, idName)
		if err != nil {
			t.Errorf("match(%q): %v", tc.key, err)
		} else if got.name != tc.want {
			t.Errorf("match(%q) = %q, want %q", tc.key, got.name, tc.want)
		}
	}

	dupes := []item{{"4ddbbffc", "Renew your plan"}, {"4ddbbffc", "t.me/bot"}}
	if got, err := match("4ddbbffc", "node", dupes, idName); err != nil || got.name != "Renew your plan" {
		t.Errorf("shared id: got %q, %v; want the first hit, nil", got.name, err)
	}

	if _, err := match("zzz", "node", items, idName); err == nil {
		t.Error("match(zzz): want an error")
	}
	if _, err := match("frankfurt", "node", items, idName); err == nil {
		t.Error("match(frankfurt): want an ambiguity error")
	}
}
