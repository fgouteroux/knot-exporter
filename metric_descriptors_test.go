package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMetricDescriptorsMaps tests the metric descriptor maps
func TestMetricDescriptorsMaps(t *testing.T) {
	// Test that the global stats descriptors map is initialized
	assert.NotNil(t, globalStatsDescriptors)

	// For mutexes, skip checking them directly to avoid copylocks
	// The fact that the code compiles and runs means they exist

	// Test that the zone stats descriptors map is initialized
	assert.NotNil(t, zoneStatsDescriptors)
}

// TestPredefinedMetricDescriptors tests the predefined metric descriptors
func TestPredefinedMetricDescriptors(t *testing.T) {
	// Test memory usage descriptor
	assert.NotNil(t, memoryUsageDesc)
	assert.Contains(t, memoryUsageDesc[0].String(), "knot_memory_usage_bytes")
	assert.Contains(t, memoryUsageDesc[1].String(), "knot_memory_usage_bytes_total")

	// Test zone serial descriptor
	assert.NotNil(t, zoneSerialDesc)
	assert.Contains(t, zoneSerialDesc[0].String(), "knot_zone_serial")
	assert.Contains(t, zoneSerialDesc[1].String(), "knot_zone_serial_total")

	// Test zone timer descriptors
	assert.NotNil(t, zoneRefreshDesc)
	assert.Contains(t, zoneRefreshDesc[0].String(), "knot_zone_refresh_seconds")

	assert.NotNil(t, zoneRetryDesc)
	assert.Contains(t, zoneRetryDesc[0].String(), "knot_zone_retry_seconds")

	assert.NotNil(t, zoneExpirationDesc)
	assert.Contains(t, zoneExpirationDesc[0].String(), "knot_zone_expiration_seconds")

	// Test zone status timer descriptors
	assert.NotNil(t, zoneStatusExpirationDesc)
	assert.Contains(t, zoneStatusExpirationDesc[0].String(), "knot_zone_status_expiration_seconds")

	assert.NotNil(t, zoneStatusRefreshDesc)
	assert.Contains(t, zoneStatusRefreshDesc[0].String(), "knot_zone_status_refresh_seconds")

	// Test build info descriptor
	assert.NotNil(t, buildInfoDesc)
	assert.Contains(t, buildInfoDesc.String(), "knot_build_info")
}

// TestDynamicDescriptorCreation tests that dynamic descriptors are created correctly
func TestDynamicDescriptorCreation(t *testing.T) {
	// Create a new global stats descriptor
	firstDesc := getGlobalStatsDescriptor("unique.test.item")
	assert.NotNil(t, firstDesc)
	assert.Contains(t, firstDesc[0].String(), "knot_stats_unique_test_item")

	// Get the same descriptor again (should be cached)
	secondDesc := getGlobalStatsDescriptor("unique.test.item")
	assert.Equal(t, firstDesc[0].String(), secondDesc[0].String())

	// Create a new zone stats descriptor
	firstZoneDesc := getZoneStatsDescriptor("unique.zone.item")
	assert.NotNil(t, firstZoneDesc)
	assert.Contains(t, firstZoneDesc[0].String(), "knot_zone_stats_unique_zone_item")

	// Get the same descriptor again (should be cached)
	secondZoneDesc := getZoneStatsDescriptor("unique.zone.item")
	assert.Equal(t, firstZoneDesc[0].String(), secondZoneDesc[0].String())
}
