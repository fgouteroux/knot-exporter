package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// TestCollectorOptions tests all combinations of collector options
func TestCollectorOptions(t *testing.T) {
	// Test all combinations of collector options
	// Each test case has the collector options and the expected behavior
	testCases := []struct {
		name              string
		collectMemInfo    bool
		collectStats      bool
		collectZoneStats  bool
		collectZoneStatus bool
		collectZoneSerial bool
		collectZoneTimers bool
	}{
		{"All options enabled", true, true, true, true, true, true},
		{"All options disabled", false, false, false, false, false, false},
		{"Only memInfo", true, false, false, false, false, false},
		{"Only global stats", false, true, false, false, false, false},
		{"Only zone stats", false, false, true, false, false, false},
		{"Only zone status", false, false, false, true, false, false},
		{"Only zone serial", false, false, false, false, true, false},
		{"Only zone timers", false, false, false, false, false, true},
	}

	// For each test case, create a collector and verify that the options are set correctly
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a collector with the specified options
			collector := newKnotCollector("/test", 1000,
				tc.collectMemInfo,
				tc.collectStats,
				tc.collectZoneStats,
				tc.collectZoneStatus,
				tc.collectZoneSerial,
				tc.collectZoneTimers,
			)

			// Verify that the options are set correctly
			assert.Equal(t, tc.collectMemInfo, collector.collectMemInfo)
			assert.Equal(t, tc.collectStats, collector.collectStats)
			assert.Equal(t, tc.collectZoneStats, collector.collectZoneStats)
			assert.Equal(t, tc.collectZoneStatus, collector.collectZoneStatus)
			assert.Equal(t, tc.collectZoneSerial, collector.collectZoneSerial)
			assert.Equal(t, tc.collectZoneTimers, collector.collectZoneTimers)

			// Create a registry and register the collector
			registry := prometheus.NewRegistry()
			registry.MustRegister(collector)

			// Collecting metrics should not panic even with options disabled
			assert.NotPanics(t, func() {
				metrics, err := registry.Gather()
				assert.NoError(t, err)

				// Should at least have the build info metric
				foundBuildInfo := false
				for _, mf := range metrics {
					if mf.GetName() == "knot_build_info" {
						foundBuildInfo = true
						break
					}
				}

				assert.True(t, foundBuildInfo, "Build info metric should be present")
			})
		})
	}
}
