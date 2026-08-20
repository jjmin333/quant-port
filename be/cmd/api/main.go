package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/server"
)

const shutdownTimeout = 30 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	// Docker stop / Ctrl-C must drain in-flight requests before the pools close.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("quant-port API listening on %s", cfg.Addr())
		if err := srv.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		log.Fatalf("run: %v", err)
	case sig := <-stop:
		log.Printf("shutdown: received %s, draining…", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("shutdown: complete")
}
