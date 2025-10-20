package main

import (
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// TestKnotCollector_Describe tests the Describe method of KnotCollector
func TestKnotCollector_Describe(t *testing.T) {
	collector := newKnotCollector("/run/knot/knot.sock", 1000, true, true, true, true, true, true)

	ch := make(chan *prometheus.Desc, 50)
	collector.Describe(ch)
	close(ch)

	descCount := 0
	for range ch {
		descCount++
	}

	// Should have at least the build info metric and several others
	assert.True(t, descCount > 1, "Expected multiple metric descriptions")
}

// TestKnotCollector_ConvertStateTime tests the convertStateTime method
func TestKnotCollector_ConvertStateTime(t *testing.T) {
	collector := newKnotCollector("/run/knot/knot.sock", 1000, true, true, true, true, true, true)

	tests := []struct {
		name     string
		timeStr  string
		expected *float64
		isNil    bool
	}{
		{"Pending state", "pending", floatPtr(0), false},
		{"Running state", "running", floatPtr(0), false},
		{"Frozen state", "frozen", floatPtr(0), false},
		{"Not scheduled", "not scheduled", nil, true},
		{"Dash", "-", nil, true},
		{"Valid positive duration", "+1h30m", floatPtr(5400), false},
		{"Valid negative duration", "-30m", floatPtr(-1800), false},
		// Update the expected value to match the actual implementation
		{"Complex duration", "+2D5h10m20s", floatPtr(191420), false}, // 2*86400 + 5*3600 + 10*60 + 20
		{"Invalid format", "invalid", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.convertStateTime(tt.timeStr)

			if tt.isNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.InDelta(t, *tt.expected, *result, 0.001)
			}
		})
	}
}

// TestKnotCollector_Collect tests the basic functionality of the Collect method
func TestKnotCollector_Collect(t *testing.T) {
	// Create a registry
	registry := prometheus.NewRegistry()

	// Create a collector with all options disabled for simpler testing
	collector := newKnotCollector("/nonexistent", 1000, false, false, false, false, false, false)

	// Register the collector
	registry.MustRegister(collector)

	// Collecting metrics should not panic even with a non-existent socket
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
}

// TestGetProcessMemory tests the getProcessMemory function
func TestGetProcessMemory(t *testing.T) {
	// Skip this test on non-Linux systems
	if _, err := os.Stat("/proc"); os.IsNotExist(err) {
		t.Skip("Skipping test on non-Linux systems (no /proc filesystem)")
	}

	// Test with self (should return > 0 if running process)
	pid := os.Getpid()
	mem := getProcessMemory(pid)

	// On normal systems, the test process should use some memory
	assert.Greater(t, mem, uint64(0), "Expected non-zero memory usage for test process")

	// Test with non-existent PID
	mem = getProcessMemory(-1)
	assert.Equal(t, uint64(0), mem, "Expected zero memory for invalid PID")
}

// TestMemoryUsage tests the memoryUsage function
func TestMemoryUsage(t *testing.T) {
	// This is hard to test directly since it depends on having knotd running
	// We'll just ensure it returns a map and doesn't panic
	usage := memoryUsage()
	assert.IsType(t, map[string]uint64{}, usage)
}

// TestSendMetrics tests the sendMetrics function
func TestSendMetrics(t *testing.T) {
	// Create a channel
	ch := make(chan prometheus.Metric, 10)

	// Create a test descriptor pair
	desc := makeDescPair("test_metric", "Test help", []string{"label"}, nil)

	// Send metrics
	sendMetrics(ch, desc, 123.45, "value")

	// Verify metrics were sent to the channel
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	// Should have 2 metrics (gauge and counter)
	assert.Equal(t, 2, metricCount, "Should have sent 2 metrics to the channel")

	// Alternative approach that doesn't try to register the metrics with a registry
	// Create a new channel and test descriptor
	ch2 := make(chan prometheus.Metric, 10)
	desc2 := makeDescPair("another_test_metric", "Another test help", []string{"label"}, nil)

	// Send metrics
	sendMetrics(ch2, desc2, 456.78, "another_value")

	// Collect metrics
	close(ch2)
	var metrics []prometheus.Metric
	for m := range ch2 {
		metrics = append(metrics, m)
	}

	// Should have 2 metrics (gauge and counter)
	assert.Equal(t, 2, len(metrics), "Should have collected 2 metrics")
}

// TestNewKnotCollector tests the newKnotCollector factory function
func TestNewKnotCollector(t *testing.T) {
	// Test with default options
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	assert.NotNil(t, collector)
	assert.Equal(t, "/test", collector.sockPath)
	assert.Equal(t, 1000, collector.timeout)
	assert.True(t, collector.collectMemInfo)
	assert.True(t, collector.collectStats)
	assert.True(t, collector.collectZoneStats)
	assert.True(t, collector.collectZoneStatus)
	assert.True(t, collector.collectZoneSerial)
	assert.True(t, collector.collectZoneTimers)

	// Test with custom options
	collector = newKnotCollector("/other", 2000, false, false, true, false, true, false)
	assert.NotNil(t, collector)
	assert.Equal(t, "/other", collector.sockPath)
	assert.Equal(t, 2000, collector.timeout)
	assert.False(t, collector.collectMemInfo)
	assert.False(t, collector.collectStats)
	assert.True(t, collector.collectZoneStats)
	assert.False(t, collector.collectZoneStatus)
	assert.True(t, collector.collectZoneSerial)
	assert.False(t, collector.collectZoneTimers)
}
