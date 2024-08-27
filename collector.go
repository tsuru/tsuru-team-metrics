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
	labelsAppMetadata = []string{"app", "team_owner", "pool", "plan"}
	labelsAppAddress  = []string{"app", "router", "address"}
	labelsAppCName    = []string{"app", "cname"}
	labelsAppUnits    = []string{"app", "process"}

	labelsService     = []string{"service", "service_instance", "team_owner", "pool", "plan"}
	serviceBindLabels = []string{"service", "service_instance", "app"}
	appMetadataDesc   = prometheus.NewDesc("tsuru_app_metadata", "tsuru app metadata.", labelsAppMetadata, nil)
	appAddressesDesc  = prometheus.NewDesc("tsuru_app_address", "tsuru app address.", labelsAppAddress, nil)
	appCNameDesc      = prometheus.NewDesc("tsuru_app_cname", "tsuru app cnames.", labelsAppCName, nil)
	appUnitsDesc      = prometheus.NewDesc("tsuru_app_units_total", "tsuru app units.", labelsAppUnits, nil)

	serviceInstanceMetadataDesc = prometheus.NewDesc("tsuru_service_instance_metadata", "tsuru service instance metadata.", labelsService, nil)
	serviceInstanceBindDesc     = prometheus.NewDesc("tsuru_service_instance_bind", "tsuru service instance bind.", serviceBindLabels, nil)
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
	ch <- appAddressesDesc
	ch <- serviceInstanceMetadataDesc
	ch <- appUnitsDesc
	ch <- appCNameDesc
}

func (p *teamsCollector) Collect(ch chan<- prometheus.Metric) {
	p.RLock()
	defer p.RUnlock()
	p.checkSync()
	for _, a := range p.apps {
		ch <- prometheus.MustNewConstMetric(appMetadataDesc, prometheus.GaugeValue, 1.0, a.Name, a.TeamOwner, a.Pool, a.Plan.Name)
	}

	routersAlreadySent := map[sentRouterAddress]bool{}
	for _, a := range p.apps {
		for _, router := range a.Routers {
			key := sentRouterAddress{a.Name, router.Name, router.Address}
			if routersAlreadySent[key] {
				continue
			}
			ch <- prometheus.MustNewConstMetric(appAddressesDesc, prometheus.GaugeValue, 1.0, a.Name, router.Name, router.Address)
			routersAlreadySent[key] = true
		}
	}
	for _, si := range p.serviceInstances {
		ch <- prometheus.MustNewConstMetric(serviceInstanceMetadataDesc, prometheus.GaugeValue, 1.0, si.ServiceName, si.Name, si.TeamOwner, si.Pool, si.PlanName)
	}

	for _, si := range p.serviceInstances {
		for _, app := range si.Apps {
			ch <- prometheus.MustNewConstMetric(serviceInstanceBindDesc, prometheus.GaugeValue, 1.0, si.ServiceName, si.Name, app)
		}
	}

	for _, a := range p.apps {
		unitsPerProcess := map[string]int{}
		for _, u := range a.Units {
			unitsPerProcess[u.Processname]++
		}

		for process, nUnits := range unitsPerProcess {
			ch <- prometheus.MustNewConstMetric(appUnitsDesc, prometheus.GaugeValue, float64(nUnits), a.Name, process)
		}
	}

	cnameAlreadySent := map[sentAppCName]bool{}

	for _, a := range p.apps {
		for _, cname := range a.Cname {
			key := sentAppCName{a.Name, cname}
			if cnameAlreadySent[key] {
				continue
			}
			ch <- prometheus.MustNewConstMetric(appCNameDesc, prometheus.GaugeValue, float64(1), a.Name, cname)
			cnameAlreadySent[key] = true
		}
	}
}

type sentRouterAddress struct {
	app     string
	router  string
	address string
}

type sentAppCName struct {
	app   string
	cname string
}
