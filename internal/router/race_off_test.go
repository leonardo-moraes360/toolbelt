//go:build !race

package router

import "time"

const waitTimeout = 30 * time.Second
