package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
)

const (
	defaultSyncInterval = 5 * time.Minute
)

var (
	labelsApp                   = []string{"app", "team_owner", "pool"}
	labelsService               = []string{"service", "service_instance", "team_owner", "pool"}
	appMetadataDesc             = prometheus.NewDesc("tsuru_app_metadata", "tsuru app metadata.", labelsApp, nil)
	serviceInstanceMetadataDesc = prometheus.NewDesc("tsuru_service_instance_metadata", "tsuru service instance metadata.", labelsService, nil)
)

type teamsCollector struct {
	sync.RWMutex

	config           config
	syncRunning      chan struct{}
	requestsLimit    chan struct{}
	lastSync         time.Time
	client           *tsuru.APIClient
	apps             []tsuru.MiniApp
	serviceInstances []tsuru.ServiceInstance
}

func newTeamsCollector(conf config, client *tsuru.APIClient) (*teamsCollector, error) {
	collector := &teamsCollector{
		client:        client,
		config:        conf,
		syncRunning:   make(chan struct{}, 1),
		requestsLimit: make(chan struct{}, conf.maxRequests),
	}
	return collector, prometheus.Register(collector)
}

func (p *teamsCollector) fetchApps() ([]tsuru.MiniApp, error) {
	apps, _, err := p.client.AppApi.AppList(context.TODO(), nil)
	return apps, err
}

func (p *teamsCollector) sync() {
	defer func() {
		p.Lock()
		p.lastSync = time.Now()
		p.Unlock()
	}()
	log.Print("[sync] listing apps")
	apps, _, err := p.client.AppApi.AppList(context.TODO(), nil)
	if err != nil {
		log.Printf("unable to fetch app list: %v", err)
		return
	}
	p.Lock()
	p.apps = apps
	p.Unlock()
	log.Print("[sync] listing instances")
	svcs, _, err := p.client.ServiceApi.InstancesList(context.TODO(), nil)
	if err != nil {
		log.Printf("unable to fetch service instance list: %v", err)
		return
	}
	newSvcs := make([]tsuru.ServiceInstance, 0, len(p.serviceInstances))
	for _, svc := range svcs {
		for _, si := range svc.ServiceInstances {
			newSvcs = append(newSvcs, si)
		}
	}
	p.Lock()
	p.serviceInstances = newSvcs
	p.Unlock()
}

func (p *teamsCollector) checkSync() {
	if time.Since(p.lastSync) > p.config.syncInterval {
		select {
		case p.syncRunning <- struct{}{}:
		default:
			// sync already in progress
			return
		}
		go func() {
			defer func() {
				log.Print("[sync] finished tsuru data sync")
				<-p.syncRunning
			}()
			log.Print("[sync] starting tsuru data sync")
			p.sync()
		}()
	}
}

func (p *teamsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- appMetadataDesc
	ch <- serviceInstanceMetadataDesc
}

func (p *teamsCollector) Collect(ch chan<- prometheus.Metric) {
	p.RLock()
	defer p.RUnlock()
	p.checkSync()
	for _, a := range p.apps {
		ch <- prometheus.MustNewConstMetric(appMetadataDesc, prometheus.GaugeValue, 1.0, a.Name, a.TeamOwner, a.Pool)
	}
	for _, si := range p.serviceInstances {
		ch <- prometheus.MustNewConstMetric(serviceInstanceMetadataDesc, prometheus.GaugeValue, 1.0, si.ServiceName, si.Name, si.TeamOwner, si.Pool)
	}
}
