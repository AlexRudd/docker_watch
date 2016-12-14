// Exporter is a prometheus exporter using multiple Factories to collect and export system metrics.

package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Namespace of metrics produced by docker_watch
const Namespace = "dw"

// Factories map of collectors to names
var Factories = make(map[string]func() (Collector, error))

// Collector Interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) (err error)
}
