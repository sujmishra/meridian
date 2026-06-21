package registry

import (
	"context"
	"time"

	"github.com/sujmishra/meridian/packages/identity"
)

// HealthChecker actively probes registered agents and updates their health status.
type HealthChecker interface {
	// CheckAll probes all registered agents and updates their Health field.
	CheckAll(ctx context.Context) error

	// Check probes a single agent and returns its current health.
	Check(ctx context.Context, agentURI identity.URI) (HealthStatus, error)
}

// HealthCheckConfig configures the health check behaviour.
type HealthCheckConfig struct {
	// Interval between health check sweeps.
	Interval time.Duration

	// Timeout per individual agent probe.
	Timeout time.Duration

	// FailThreshold is the number of consecutive failures before marking an agent degraded.
	FailThreshold int
}
