package fakes

import (
	"time"

	aiven "github.com/aiven/aiven-go-client"
)

type FakeFetcher struct {
	shouldReturn []aiven.Service
}

func NewFakeFetcher() *FakeFetcher {
	return &FakeFetcher{shouldReturn: make([]aiven.Service, 0)}
}

func (f *FakeFetcher) Start()                         {}
func (f *FakeFetcher) Stop()                          {}
func (f *FakeFetcher) SetInterval(_ time.Duration)    {}
func (f *FakeFetcher) Services() []aiven.Service      { return f.shouldReturn }
func (f *FakeFetcher) ShouldReturn(s []aiven.Service) { f.shouldReturn = s }
