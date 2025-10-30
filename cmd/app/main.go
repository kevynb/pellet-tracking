// Package main bootstraps the pellets tracker HTTP and TSnet servers.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pellets-tracker/internal/config"
	httpserver "pellets-tracker/internal/http"
	"pellets-tracker/internal/store"
	tsnetserver "pellets-tracker/internal/tsnet"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	dataStore, err := store.NewJSONStore(cfg.DataFile, cfg.BackupDir)
	if err != nil {
		log.Fatalf("failed to initialize datastore: %v", err)
	}

	apiServer := httpserver.NewServer(dataStore)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      apiServer.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	listener, cleanup, listenAddr, err := prepareListener(cfg)
	if err != nil {
		log.Fatalf("failed to prepare listener: %v", err)
	}
	defer func() {
		if cleanup != nil {
			if err := cleanup(); err != nil && !errors.Is(err, net.ErrClosed) {
				log.Printf("listener cleanup error: %v", err)
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("pellets tracker listening on %s (tsnet=%v)", listenAddr, cfg.TsnetEnabled)
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}

	log.Println("server stopped cleanly")
}

func prepareListener(cfg *config.Config) (net.Listener, func() error, string, error) {
	if !cfg.TsnetEnabled {
		ln, err := net.Listen("tcp", cfg.ListenAddr)
		if err != nil {
			return nil, nil, "", err
		}
		return ln, ln.Close, cfg.ListenAddr, nil
	}

	tsServer, err := tsnetserver.New(tsnetserver.Config{
		Hostname: cfg.TsnetHostname,
		Dir:      cfg.TsnetDir,
		AuthKey:  cfg.TsnetAuthKey,
		Listen:   cfg.TsnetListenAddr,
	})
	if err != nil {
		return nil, nil, "", err
	}
	ln, err := tsServer.Listen()
	if err != nil {
		tsServer.Close()
		return nil, nil, "", err
	}
	return ln, tsServer.Close, cfg.TsnetListenAddr, nil
}
