package resolver

import (
	"net"
)

func init() {
	initMetrics()
}

type Resolver interface {
	Resolve(string) ([]net.IP, error)
}

type resolver struct{}

func (r *resolver) Resolve(hostname string) ([]net.IP, error) {
	ips, err := net.LookupIP(hostname)

	ResolverResolvesTotal.Inc()

	if err != nil {
		ResolverResolveFailuresTotal.Inc()
	}

	return ips, err
}

func NewResolver() Resolver {
	return &resolver{}
}
