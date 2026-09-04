//go:build !darwin

package owner

func File(string) error { return nil }
