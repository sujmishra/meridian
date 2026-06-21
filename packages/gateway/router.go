package gateway

import (
	"context"
	"errors"

	"github.com/sujmishra/meridian/packages/registry"
)

var (
	ErrNoAgentAvailable = errors.New("gateway: no healthy agent found for capability")
	ErrNoAdapter        = errors.New("gateway: no adapter registered for protocol pair")
)

// Router selects a target agent record from the registry for a given request.
// It implements capability-path-based routing with health filtering.
type Router interface {
	// Select returns a healthy agent record matching the request's capability path
	// and target protocol preference.
	Select(ctx context.Context, req Request) (registry.Record, error)
}

// LoadBalancer chooses among multiple healthy candidates.
type LoadBalancer interface {
	// Pick selects one record from the candidates list.
	Pick(candidates []registry.Record) (registry.Record, error)
}

// LoadBalancerFunc is a function that implements LoadBalancer.
type LoadBalancerFunc func(candidates []registry.Record) (registry.Record, error)

func (f LoadBalancerFunc) Pick(candidates []registry.Record) (registry.Record, error) {
	return f(candidates)
}

// RoundRobin returns a LoadBalancer that cycles through candidates in order.
func RoundRobin() LoadBalancer {
	var i int
	return LoadBalancerFunc(func(candidates []registry.Record) (registry.Record, error) {
		if len(candidates) == 0 {
			return registry.Record{}, ErrNoAgentAvailable
		}
		r := candidates[i%len(candidates)]
		i++
		return r, nil
	})
}
