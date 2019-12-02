package fakes

import (
	"net"
)

type FakeResolver struct {
	shouldReturnIPs   []net.IP
	shouldReturnError error
}

func NewFakeResolver() *FakeResolver {
	return &FakeResolver{
		shouldReturnIPs:   make([]net.IP, 0),
		shouldReturnError: nil,
	}
}

func (r *FakeResolver) Resolve(hostname string) ([]net.IP, error) {
	if r.shouldReturnError != nil {
		return make([]net.IP, 0), r.shouldReturnError
	}

	return r.shouldReturnIPs, nil
}

func (r *FakeResolver) ShouldReturnIPs(ips []net.IP) {
	r.shouldReturnIPs = ips
}

func (r *FakeResolver) ShouldReturnError(err error) {
	r.shouldReturnError = err
}
