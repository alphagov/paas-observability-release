package discoverer

import (
	"net"
)

type prometheusTargetConfig struct {
	Targets []net.IP          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}
