package main

import (
	"log"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	log.Printf("quant-port API listening on %s", cfg.Addr())
	if err := srv.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
