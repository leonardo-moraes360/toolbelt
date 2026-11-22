//go:build race

package router

import "time"

// waitTimeout is how long a test polls for an asynchronous condition; the
// race detector slows the router several times over.
const waitTimeout = 90 * time.Second
