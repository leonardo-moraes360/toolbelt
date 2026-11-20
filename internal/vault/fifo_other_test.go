//go:build !unix

package vault

import "testing"

func mkfifo(*testing.T, string) {}
