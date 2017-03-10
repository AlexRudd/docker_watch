package collector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// DockerInspectCollector orchestrates the collectors for Docker containers
type DockerInspectCollector struct {
	mtx        sync.RWMutex
	containers []types.Container
}

func init() {
	Factories["inspect"] = NewDockerInspectCollector
}

// NewDockerInspectCollector instanstiates DockerInspectCollector
func NewDockerInspectCollector() (Collector, error) {
	return &DockerInspectCollector{
		mtx: sync.RWMutex{},
	}, nil
}

// Update - checks for new/departed containers and scrapes them
func (c *DockerInspectCollector) Update(ch chan<- prometheus.Metric) (err error) {

	c.updateContainerList()

	var wg sync.WaitGroup
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, container := range c.containers {
		wg.Add(1)
		go func(container types.Container) {
			defer wg.Done()
			s := c.scrapeDockerInspect(container)
			if s == nil {
				return
			}
			// set container name
			var labels = make(prometheus.Labels)
			labels["name"] = strings.TrimPrefix(s.Name, "/")
			labels["id"] = container.ID
			for lk, lv := range container.Labels {
				labels[lk] = lv
			}

			// General Info
			var tLabels = make(prometheus.Labels)
			for k, v := range labels {
				tLabels[k] = v
			}
			tLabels["image"] = s.Config.Image
			tLabels["image_sha"] = s.Image
			m := prometheus.NewCounter(prometheus.CounterOpts{
				Namespace:   Namespace,
				Subsystem:   "inspect",
				Name:        string("info"),
				Help:        fmt.Sprintf("The image name, hash, container id"),
				ConstLabels: tLabels,
			})
			m.Set(float64(1))
			m.Collect(ch)
			
			// created at
			t, err := time.Parse(time.RFC3339Nano, s.Created)
			if err != nil {
				log.Warnf("Could not parse created at timestamp (%s): %s", s.Created, err.Error())
			} else {
				tLabels = make(prometheus.Labels)
				for k, v := range labels {
					tLabels[k] = v
				}
				tLabels["date_string"] = t.String()
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "inspect",
					Name:        string("created_timestamp"),
					Help:        fmt.Sprintf("The time the container image was created as unix timestamp"),
					ConstLabels: tLabels,
				})
				//set and collect
				m.Set(float64(t.Unix()))
				m.Collect(ch)
			}

			// started at
			t, err = time.Parse(time.RFC3339Nano, s.State.StartedAt)
			if err != nil {
				log.Warnf("Could not parse started at timestamp (%s): %s", s.Created, err.Error())
			} else {
				tLabels["date_string"] = t.String()
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "inspect",
					Name:        string("started_timestamp"),
					Help:        fmt.Sprintf("The time the container was started as unix timestamp"),
					ConstLabels: tLabels,
				})
				tLabels["date_string"] = t.String()
				m.Set(float64(t.Unix()))
				m.Collect(ch)
			}

			// finished at
			t, err = time.Parse(time.RFC3339Nano, s.State.StartedAt)
			if err != nil {
				log.Warnf("Could not parse finished at timestamp (%s): %s", s.Created, err.Error())
			} else {
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "inspect",
					Name:        string("finished_timestamp"),
					Help:        fmt.Sprintf("The time the container was stopped as unix timestamp"),
					ConstLabels: tLabels,
				})
				//set and collect
				tLabels["date_string"] = t.String()
				m.Set(float64(t.Unix()))
				m.Collect(ch)
			}

			// state?
			tLabels = make(prometheus.Labels)
			for k, v := range labels {
				tLabels[k] = v
			}
			tLabels["status"] = s.State.Status
			m = prometheus.NewCounter(prometheus.CounterOpts{
				Namespace:   Namespace,
				Subsystem:   "inspect",
				Name:        string("state"),
				Help:        fmt.Sprintf("The current state of the container"),
				ConstLabels: tLabels,
			})
			m.Set(float64(1))
			m.Collect(ch)

			// PID
			m = prometheus.NewCounter(prometheus.CounterOpts{
				Namespace:   Namespace,
				Subsystem:   "inspect",
				Name:        string("process_id"),
				Help:        fmt.Sprintf("The PID of the dockerized process"),
				ConstLabels: labels,
			})
			m.Set(float64(s.State.Pid))
			m.Collect(ch)
		}(container)
	}

	wg.Wait()
	return nil
}

func (c *DockerInspectCollector) updateContainerList() {
	// Update - checks for new/departed containers and scrapes them
	log.Debugf("Fetching list of locally running containers")
	freshContainers, err := getContainerList()
	if err != nil {
		return
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.containers = freshContainers
}

func (c *DockerInspectCollector) scrapeDockerInspect(container types.Container) *types.ContainerJSON {
	log.Debugf("Scraping container stats for %s", container.Names[0])
	cli, err := getDockerClient()
	if err != nil {
		log.Errorf("Failed to create Docker api client: %s", err.Error())
		return nil
	}
	rc, err := cli.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		log.Errorf("Failed to inspect docker container %s: %s", container.Names[0], err.Error())
		return nil
	}

	return &rc
}
