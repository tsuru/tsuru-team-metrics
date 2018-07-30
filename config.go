package main

import (
	"os"
	"strconv"
	"time"
)

type config struct {
	port         string
	tsuruHost    string
	tsuruToken   string
	syncInterval time.Duration
	maxRequests  int
}

func readConfig() (config, error) {
	syncInterval, _ := time.ParseDuration(os.Getenv("SYNC_INTERVAL"))
	maxRequests, _ := strconv.Atoi(os.Getenv("MAX_REQUESTS"))
	conf := config{
		port:         os.Getenv("PORT"),
		tsuruHost:    os.Getenv("TSURU_HOST"),
		tsuruToken:   os.Getenv("TSURU_TOKEN"),
		syncInterval: syncInterval,
		maxRequests:  maxRequests,
	}
	if conf.syncInterval == 0 {
		conf.syncInterval = defaultSyncInterval
	}
	if conf.port == "" {
		conf.port = "19283"
	}
	return conf, nil
}
