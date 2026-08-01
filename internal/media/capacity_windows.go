//go:build windows

package media

func Capacity(string) (int64, int64, error) { return 0, 0, nil }
