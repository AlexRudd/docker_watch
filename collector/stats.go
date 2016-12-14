package collector

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// DockerStatsCollector orchestrates the collectors for Docker containers
type DockerStatsCollector struct {
	mtx        sync.RWMutex
	containers []types.Container
}

func init() {
	Factories["stats"] = NewDockerStatsCollector
}

// NewDockerStatsCollector instanstiates DockerStatsCollector
func NewDockerStatsCollector() (Collector, error) {
	return &DockerStatsCollector{
		mtx: sync.RWMutex{},
	}, nil
}

// Update - checks for new/departed containers and scrapes them
func (c *DockerStatsCollector) Update(ch chan<- prometheus.Metric) (err error) {

	c.updateContainerList()

	var wg sync.WaitGroup
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, container := range c.containers {
		wg.Add(1)
		go func(container types.Container) {
			defer wg.Done()
			s := c.scrapeDockerStats(container)
			if s == nil {
				return
			}
			// set container name
			var labels = make(prometheus.Labels)
			labels["name"] = strings.TrimPrefix(container.Names[0], "/")
			for lk, lv := range container.Labels {
				labels[lk] = lv
			}
			// build new cpu counter metric
			m := prometheus.NewCounter(prometheus.CounterOpts{
				Namespace:   Namespace,
				Subsystem:   "stats",
				Name:        string("cpu_usage_total_nanoseconds"),
				Help:        fmt.Sprintf("Total CPU time consumed in nanoseconds"),
				ConstLabels: labels,
			})
			//set and collect
			m.Set(float64(s.CPUStats.CPUUsage.TotalUsage))
			m.Collect(ch)

			// build new cpu throttle time counter metric
			m = prometheus.NewCounter(prometheus.CounterOpts{
				Namespace:   Namespace,
				Subsystem:   "stats",
				Name:        string("cpu_throttled_time_total_nanoseconds"),
				Help:        fmt.Sprintf("Aggregate time the container was throttled for in nanoseconds."),
				ConstLabels: labels,
			})
			//set and collect
			m.Set(float64(s.CPUStats.ThrottlingData.ThrottledTime))
			m.Collect(ch)

			// build new cpu throttled periods counter metric
			m = prometheus.NewCounter(prometheus.CounterOpts{
				Namespace:   Namespace,
				Subsystem:   "stats",
				Name:        string("cpu_throttled_periods_total"),
				Help:        fmt.Sprintf("Number of periods when the container hits its throttling limit."),
				ConstLabels: labels,
			})
			//set and collect
			m.Set(float64(s.CPUStats.ThrottlingData.ThrottledPeriods))
			m.Collect(ch)

			// build new memory gauge metric
			m = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   Namespace,
				Subsystem:   "stats",
				Name:        string("memory_usage_bytes"),
				Help:        fmt.Sprintf("Memory usage in bytes"),
				ConstLabels: labels,
			})
			//set and collect
			m.Set(float64(s.MemoryStats.Usage))
			m.Collect(ch)

			// build new memory limit gauge metric
			m = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   Namespace,
				Subsystem:   "stats",
				Name:        string("memory_limit_bytes"),
				Help:        fmt.Sprintf("Memory limit in bytes"),
				ConstLabels: labels,
			})
			//set and collect
			m.Set(float64(s.MemoryStats.Limit))
			m.Collect(ch)

			for _, io := range s.BlkioStats.IoServiceBytesRecursive {
				// add labels
				labels["dev_major"] = strconv.FormatUint(io.Major, 10)
				labels["dev_minor"] = strconv.FormatUint(io.Minor, 10)
				labels["operation"] = strings.ToLower(io.Op)
				// build new block io counter metric
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("blkio_op_total_bytes"),
					Help:        fmt.Sprintf("Block IO ops"),
					ConstLabels: labels,
				})
				//set and collect
				m.Set(float64(io.Value))
				m.Collect(ch)
			}
			delete(labels, "dev_major")
			delete(labels, "dev_minor")
			delete(labels, "operation")

			for dev, netio := range s.Networks {
				// add labels
				labels["device"] = dev

				// RECIEVED
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_rx_total_bytes"),
					Help:        fmt.Sprintf("Network bytes recieved by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.RxBytes))
				m.Collect(ch)

				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_rx_total_packets"),
					Help:        fmt.Sprintf("Network packets recieved by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.RxPackets))
				m.Collect(ch)

				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_rx_total_errors"),
					Help:        fmt.Sprintf("Network errors recieved by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.RxErrors))
				m.Collect(ch)
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_rx_total_dropped"),
					Help:        fmt.Sprintf("Network dropped recieved by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.RxDropped))
				m.Collect(ch)

				// TRANSMITTED
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_tx_total_bytes"),
					Help:        fmt.Sprintf("Network bytes transmitted by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.TxBytes))
				m.Collect(ch)

				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_tx_total_packets"),
					Help:        fmt.Sprintf("Network packets transmitted by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.TxPackets))
				m.Collect(ch)

				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_tx_total_errors"),
					Help:        fmt.Sprintf("Network errors transmitted by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.TxErrors))
				m.Collect(ch)
				m = prometheus.NewCounter(prometheus.CounterOpts{
					Namespace:   Namespace,
					Subsystem:   "stats",
					Name:        string("network_tx_total_dropped"),
					Help:        fmt.Sprintf("Network dropped transmitted by device"),
					ConstLabels: labels,
				})
				m.Set(float64(netio.TxDropped))
				m.Collect(ch)
			}

		}(container)
	}

	wg.Wait()
	return nil
}

func (c *DockerStatsCollector) updateContainerList() {
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

func (c *DockerStatsCollector) scrapeDockerStats(container types.Container) *types.StatsJSON {
	log.Debugf("Scraping container stats for %s", container.Names[0])
	cli, err := getDockerClient()
	if err != nil {
		log.Errorf("Failed to create Docker api client: %s", err.Error())
		return nil
	}
	rc, err := cli.ContainerStats(context.Background(), container.ID, false)
	if err != nil {
		log.Errorf("Failed to fetch docker container stats for %s: %s", container.Names[0], err.Error())
		return nil
	}
	defer rc.Body.Close()
	decoder := json.NewDecoder(rc.Body)
	var stats types.StatsJSON
	err = decoder.Decode(&stats)
	if err != nil {
		log.Errorf("Couldn't decode stats json from container %s: %s", container.Names[0], err.Error())
		return nil
	}

	return &stats
}
