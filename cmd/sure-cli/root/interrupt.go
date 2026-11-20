package root

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// interruptContext is cancelled by the first Ctrl-C or SIGTERM instead of
// the process being killed, so a command writing files fails as canceled
// and removes its partial ones. A second one kills the process as usual.
func interruptContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop)
	return ctx, stop
}
