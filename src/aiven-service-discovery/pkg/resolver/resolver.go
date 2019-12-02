package resolver

import (
	"net"
)

type Resolver interface {
	Resolve(string) ([]net.IP, error)
}

type resolver struct{}

func (r *resolver) Resolve(hostname string) ([]net.IP, error) {
	return net.LookupIP(hostname)
}

func NewResolver() Resolver {
	return &resolver{}
}
