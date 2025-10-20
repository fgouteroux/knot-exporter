package main

// Helper functions used across multiple test files

// floatPtr creates a pointer to a float64 value
// Used in several test files for testing time conversion
func floatPtr(f float64) *float64 {
	return &f
}
