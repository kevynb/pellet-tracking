// Package tsnet provides helpers to expose the pellets tracker over a Tailscale
// tsnet listener.
package tsnet

import (
	"fmt"
	"net"

	"tailscale.com/tsnet"
)

// Config defines the TSnet server parameters.
type Config struct {
	Hostname string
	Dir      string
	AuthKey  string
	Listen   string
}

// Server manages the lifecycle of a tsnet-backed listener.
type Server struct {
	cfg    Config
	server *tsnet.Server
}

// New constructs a Server from the provided configuration.
func New(cfg Config) (*Server, error) {
	if cfg.Listen == "" {
		cfg.Listen = ":443"
	}
	if cfg.Hostname == "" {
		cfg.Hostname = "pellets"
	}
	ts := &tsnet.Server{
		Dir:      cfg.Dir,
		Hostname: cfg.Hostname,
	}
	if cfg.AuthKey != "" {
		ts.AuthKey = cfg.AuthKey
	}
	return &Server{cfg: cfg, server: ts}, nil
}

// Listen opens the configured TSnet listener for HTTPS traffic.
func (s *Server) Listen() (net.Listener, error) {
	if s.server == nil {
		return nil, fmt.Errorf("tsnet server not initialised")
	}
	return s.server.Listen("tcp", s.cfg.Listen)
}

// Close releases the underlying TSnet server resources.
func (s *Server) Close() error {
	if s.server == nil {
		return nil
	}
	return s.server.Close()
}
