//go:build !unix

package lock

func File(string) (unlock func(), err error) { return func() {}, nil }
