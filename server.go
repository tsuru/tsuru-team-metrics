package main

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type server struct {
	config config
}

func (s *server) start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)
	addr := "0.0.0.0:" + s.config.port
	log.Printf("Listening on %v", addr)
	return http.ListenAndServe(addr, mux)
}
