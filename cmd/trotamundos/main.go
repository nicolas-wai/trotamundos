// Comando trotamundos: raíz de composición que arma una región completa (AD §3.1). [ADR-21]
// Fase 2: LoadBalancer → N réplicas → cola de prioridades → nodo de datos primario/pasivo (src/region).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/region"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuración inválida: %v", err)
	}
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config.RegionConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// [Prototipo] HTTP local simulando HTTPS/TLS.
	reg, err := region.New(cfg, region.Options{})
	if err != nil {
		return err
	}
	defer reg.Close()

	var servers []*http.Server
	var targets []*url.URL
	for i, port := range cfg.ReplicaPorts() {
		servers = append(servers, &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: reg.Replicas[i].Handler()})
		targets = append(targets, &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", port)})
	}
	servers = append(servers, &http.Server{Addr: fmt.Sprintf(":%d", cfg.LoadBalancerPort), Handler: reg.NewLoadBalancer(targets)})
	reg.Start(ctx)

	errCh := make(chan error, len(servers))
	for _, s := range servers {
		go func() {
			log.Printf("escuchando en %s", s.Addr)
			if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	select {
	case <-ctx.Done():
	case err := <-errCh:
		stop()
		shutdown(servers)
		return err
	}
	shutdown(servers)
	return nil
}

func shutdown(servers []*http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range servers {
		_ = s.Shutdown(ctx)
	}
}
