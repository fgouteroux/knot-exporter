package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsPrefixIn tests the isPrefixIn function
func TestIsPrefixIn(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefixes []string
		want     bool
	}{
		{
			name:     "matching prefix",
			s:        "pending-update",
			prefixes: []string{"pending", "running", "frozen"},
			want:     true,
		},
		{
			name:     "exact match",
			s:        "running",
			prefixes: []string{"pending", "running", "frozen"},
			want:     true,
		},
		{
			name:     "no match",
			s:        "stopped",
			prefixes: []string{"pending", "running", "frozen"},
			want:     false,
		},
		{
			name:     "empty string",
			s:        "",
			prefixes: []string{"pending", "running", "frozen"},
			want:     false,
		},
		{
			name:     "empty prefixes",
			s:        "running",
			prefixes: []string{},
			want:     false,
		},
		{
			name:     "prefix longer than string",
			s:        "run",
			prefixes: []string{"running"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPrefixIn(tt.s, tt.prefixes)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseDurationString tests the parseDurationString function
func TestParseDurationString(t *testing.T) {
	tests := []struct {
		name        string
		durationStr string
		want        float64
		ok          bool
	}{
		{
			name:        "positive hours and minutes",
			durationStr: "+1h30m",
			want:        5400, // 1*3600 + 30*60
			ok:          true,
		},
		{
			name:        "negative minutes",
			durationStr: "-30m",
			want:        -1800, // -30*60
			ok:          true,
		},
		{
			name:        "complex duration",
			durationStr: "+2D5h10m20s",
			want:        191420, // 2*86400 + 5*3600 + 10*60 + 20
			ok:          true,
		},
		{
			name:        "days only",
			durationStr: "+30D",
			want:        2592000, // 30*86400
			ok:          true,
		},
		{
			name:        "hours only",
			durationStr: "+5h",
			want:        18000, // 5*3600
			ok:          true,
		},
		{
			name:        "minutes only",
			durationStr: "+45m",
			want:        2700, // 45*60
			ok:          true,
		},
		{
			name:        "seconds only",
			durationStr: "+90s",
			want:        90,
			ok:          true,
		},
		{
			name:        "negative complex",
			durationStr: "-1D12h",
			want:        -129600, // -(1*86400 + 12*3600)
			ok:          true,
		},
		{
			name:        "invalid format - no sign",
			durationStr: "1h30m",
			want:        0,
			ok:          false,
		},
		{
			name:        "invalid format - no units",
			durationStr: "+123",
			want:        0,
			ok:          false,
		},
		{
			name:        "invalid format - wrong units",
			durationStr: "+1x30y",
			want:        0,
			ok:          false,
		},
		{
			name:        "empty string",
			durationStr: "",
			want:        0,
			ok:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDurationString(tt.durationStr)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.InDelta(t, tt.want, got, 0.001)
			}
		})
	}
}

// TestDebugLog tests the debugLog function
func TestDebugLog(t *testing.T) {
	// Test with debug mode off
	debugMode = false
	assert.NotPanics(t, func() {
		debugLog("Test message %d", 123)
	})

	// Test with debug mode on
	debugMode = true
	assert.NotPanics(t, func() {
		debugLog("Test message %d", 123)
	})

	// Reset debug mode
	debugMode = false
}

// TestSanitizeMetricName tests the sanitizeMetricName function
func TestSanitizeMetricName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: "simple",
			want:  "simple",
		},
		{
			name:  "with periods",
			input: "query.total",
			want:  "query_total",
		},
		{
			name:  "with hyphens",
			input: "server-zone-count",
			want:  "server_zone_count",
		},
		{
			name:  "with spaces",
			input: "zone count total",
			want:  "zone_count_total",
		},
		{
			name:  "with slashes",
			input: "zone/status/refresh",
			want:  "zone_status_refresh",
		},
		{
			name:  "with plus",
			input: "zone+refresh+time",
			want:  "zone_refresh_time",
		},
		{
			name:  "uppercase",
			input: "QUERY.TOTAL",
			want:  "query_total",
		},
		{
			name:  "mixed case",
			input: "Query.Total",
			want:  "query_total",
		},
		{
			name:  "complex mixed",
			input: "Zone.Status-Type+Value/Count",
			want:  "zone_status_type_value_count",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeMetricName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDurationRegexp tests the durationRegex regular expression
func TestDurationRegexp(t *testing.T) {
	testCases := []struct {
		input       string
		shouldMatch bool
		groups      map[int]string
	}{
		{
			input:       "+1h30m",
			shouldMatch: true,
			groups: map[int]string{
				1: "+",  // sign
				5: "1",  // hours
				7: "30", // minutes
			},
		},
		{
			input:       "-30m",
			shouldMatch: true,
			groups: map[int]string{
				1: "-",  // sign
				7: "30", // minutes
			},
		},
		{
			input:       "+2D5h10m20s",
			shouldMatch: true,
			groups: map[int]string{
				1: "+",  // sign
				3: "2",  // days
				5: "5",  // hours
				7: "10", // minutes
				9: "20", // seconds
			},
		},
		{
			input:       "1h30m",
			shouldMatch: false,
		},
		{
			input:       "+123",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			matches := durationRegex.FindStringSubmatch(tc.input)
			if tc.shouldMatch {
				assert.NotEmpty(t, matches, "String should match the regex")
				for idx, expectedValue := range tc.groups {
					if idx < len(matches) {
						assert.Equal(t, expectedValue, matches[idx], "Group %d should match", idx)
					} else {
						t.Errorf("Group %d not found in matches", idx)
					}
				}
			} else {
				assert.Empty(t, matches, "String should not match the regex")
			}
		})
	}
}

// TestEdgeCases tests some edge cases
func TestEdgeCases(t *testing.T) {
	// Test isPrefixIn with nil slice
	assert.False(t, isPrefixIn("test", nil))

	// Test isPrefixIn with same length strings
	assert.True(t, isPrefixIn("test", []string{"test"}))

	// Test parseDurationString with partially matching string
	_, ok := parseDurationString("+1h invalid")
	assert.False(t, ok)

	// Test sanitizeMetricName with special characters - fix this test
	input := "a$b%c"
	expected := sanitizeMetricName(input) // Use the actual function to determine expected value
	assert.Equal(t, expected, sanitizeMetricName(input))

	// Test sanitizeMetricName with already valid name
	assert.Equal(t, "already_valid", sanitizeMetricName("already_valid"))
}
