package observability_test

import (
	"fmt"
	"testing"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

func TestActiveAgentsTotalWithRegistryConnectedCount(t *testing.T) {
	reg := withIsolatedRegistry(t)

	r := registry.New()
	for i := range 5 {
		if _, err := r.Register(fmt.Sprintf("agent-%d", i), nil, nil); err != nil {
			t.Fatalf("register agent-%d: %v", i, err)
		}
	}

	_ = observability.NewMetrics(r)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var got float64
	for _, mf := range mfs {
		if mf.GetName() != "active_agents_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			got = m.GetGauge().GetValue()
		}
	}
	if got != 5 {
		t.Errorf("active_agents_total = %v, want 5", got)
	}
}
