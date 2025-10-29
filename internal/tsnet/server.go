package tsnet

import (
	"fmt"
	"net"

	"tailscale.com/tsnet"
)

type Config struct {
	Hostname string
	Dir      string
	AuthKey  string
	Listen   string
}

type Server struct {
	cfg    Config
	server *tsnet.Server
}

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

func (s *Server) Listen() (net.Listener, error) {
	if s.server == nil {
		return nil, fmt.Errorf("tsnet server not initialised")
	}
	return s.server.Listen("tcp", s.cfg.Listen)
}

func (s *Server) Close() error {
	if s.server == nil {
		return nil
	}
	return s.server.Close()
}
