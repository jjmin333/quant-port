package safego

import (
	"context"
	"log"
	"runtime/debug"
)

// Go runs fn in a new goroutine and logs any panic with a stack trace instead
// of crashing the process. Use for long-lived background workers started from
// the API process (pollers, schedulers, import jobs).
func Go(name string, fn func()) {
	go func() {
		defer logPanic(name)
		fn()
	}()
}

// GoCtx is like Go but passes a context that callers typically derive from the
// server shutdown root so the goroutine can exit when the API drains.
func GoCtx(ctx context.Context, name string, fn func(context.Context)) {
	go func() {
		defer logPanic(name)
		fn(ctx)
	}()
}

func logPanic(name string) {
	if rec := recover(); rec != nil {
		if name == "" {
			name = "background"
		}
		log.Printf("panic recovered in %s: %v\n%s", name, rec, debug.Stack())
	}
}

// Call runs fn and converts a panic into a logged error return value. Useful
// inside worker pools where one bad item must not abort the whole batch.
func Call(name string, fn func()) (panicked bool) {
	defer func() {
		if rec := recover(); rec != nil {
			panicked = true
			if name == "" {
				name = "call"
			}
			log.Printf("panic recovered in %s: %v\n%s", name, rec, debug.Stack())
		}
	}()
	fn()
	return false
}
