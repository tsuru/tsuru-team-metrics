package main

import (
	"github.com/tsuru/go-tsuruclient/pkg/client"
	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
)

func createTsuruClient(conf config) (*tsuru.APIClient, error) {
	return client.ClientFromEnvironment(&tsuru.Configuration{
		BasePath: conf.tsuruHost,
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
