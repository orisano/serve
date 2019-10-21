package serve

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/activation"
	"golang.org/x/xerrors"
)

func ListenAndServe(s *http.Server, shutdownTimeout time.Duration) error {
	lis, err := listener(s)
	if err != nil {
		return xerrors.Errorf("listen: %w", err)
	}

	errCh := make(chan error)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		s.SetKeepAlivesEnabled(false)
		errCh <- s.Shutdown(ctx)
	}()
	if err := s.Serve(lis); err != http.ErrServerClosed {
		return xerrors.Errorf("serve: %w", err)
	}
	if err := <-errCh; err != nil {
		return xerrors.Errorf("shutdown: %w", err)
	}
	return nil
}

func listener(s *http.Server) (net.Listener, error) {
	listeners, _ := activation.Listeners()
	if len(listeners) != 0 {
		return listeners[0], nil
	}
	addr := s.Addr
	if addr == "" {
		addr = ":http"
	}
	return net.Listen("tcp", addr)
}
