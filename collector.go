package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
)

const (
	defaultSyncInterval = 15 * time.Minute
	defaultMaxRequests  = 10
)

var (
	labelsApp                   = []string{"app", "team_owner"}
	labelsService               = []string{"service", "service_instance", "team_owner"}
	appMetadataDesc             = prometheus.NewDesc("tsuru_app_metadata", "tsuru app metadata.", labelsApp, nil)
	serviceInstanceMetadataDesc = prometheus.NewDesc("tsuru_service_instance_metadata", "tsuru service instance metadata.", labelsService, nil)
)

type serviceInstanceData struct {
	tsuru.ServiceInstanceInfo
	serviceName         string
	serviceInstanceName string
}

type teamsCollector struct {
	sync.RWMutex

	config           config
	syncRunning      chan struct{}
	requestsLimit    chan struct{}
	lastSync         time.Time
	client           *tsuru.APIClient
	apps             []tsuru.MiniApp
	serviceInstances map[string]serviceInstanceData
}

func newTeamsCollector(conf config, client *tsuru.APIClient) (*teamsCollector, error) {
	collector := &teamsCollector{
		client:           client,
		config:           conf,
		syncRunning:      make(chan struct{}, 1),
		requestsLimit:    make(chan struct{}, conf.maxRequests),
		serviceInstances: make(map[string]serviceInstanceData),
	}
	return collector, prometheus.Register(collector)
}

func (p *teamsCollector) fetchApps() ([]tsuru.MiniApp, error) {
	apps, _, err := p.client.AppApi.AppList(context.TODO(), nil)
	return apps, err
}

func serviceKey(s, si string) string {
	return fmt.Sprintf("%s|%s", s, si)
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
	wg := sync.WaitGroup{}
	for _, svc := range svcs {
		for _, instance := range svc.Instances {
			si := serviceInstanceData{
				serviceName:         svc.Service,
				serviceInstanceName: instance,
			}
			p.getServiceInstance(&wg, si)
		}
	}
	wg.Wait()
}

func (p *teamsCollector) getServiceInstance(wg *sync.WaitGroup, si serviceInstanceData) {
	wg.Add(1)
	p.requestsLimit <- struct{}{}
	go func() {
		defer wg.Done()
		defer func() { <-p.requestsLimit }()
		log.Printf("[sync] getting service instance %v - %v", si.serviceName, si.serviceInstanceName)
		siInfo, _, err := p.client.ServiceApi.InstanceGet(context.TODO(), si.serviceName, si.serviceInstanceName)
		if err != nil {
			log.Printf("[sync] unable to fetch service instance info for %q - %q: %v", si.serviceName, si.serviceInstanceName, err)
			return
		}
		si.ServiceInstanceInfo = siInfo
		p.Lock()
		p.serviceInstances[serviceKey(si.serviceName, si.serviceInstanceName)] = si
		p.Unlock()
	}()
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
		ch <- prometheus.MustNewConstMetric(appMetadataDesc, prometheus.GaugeValue, 1.0, a.Name, a.TeamOwner)
	}
	for _, si := range p.serviceInstances {
		ch <- prometheus.MustNewConstMetric(serviceInstanceMetadataDesc, prometheus.GaugeValue, 1.0, si.serviceName, si.serviceInstanceName, si.Teamowner)
	}
}
