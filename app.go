package main

import (
	"net"
	"net/http"
	"time"

	"github.com/tsuru/go-tsuruclient/pkg/client"
	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
)

func createTsuruClient(conf config) (*tsuru.APIClient, error) {
	dialTimeout := 10 * time.Second
	maxIdle := 100
	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
	}
	cli := &http.Client{
		Transport: &http.Transport{
			Dial:                dialer.Dial,
			TLSHandshakeTimeout: dialTimeout,
			MaxIdleConnsPerHost: maxIdle,
			MaxIdleConns:        maxIdle,
			IdleConnTimeout:     20 * time.Second,
		},
		Timeout: time.Minute,
	}
	return client.ClientFromEnvironment(&tsuru.Configuration{
		BasePath:   conf.tsuruHost,
		HTTPClient: cli,
	})
}

func startApp() error {
	conf, err := readConfig()
	if err != nil {
		return err
	}
	client, err := createTsuruClient(conf)
	if err != nil {
		return err
	}
	_, err = newTeamsCollector(conf, client)
	if err != nil {
		return err
	}
	s := server{
		config: conf,
	}
	return s.start()
}
