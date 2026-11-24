package identity

import "testing"

func TestValidPanelVisualization(t *testing.T) {
	valid := []string{"timeseries", "stat", "table", "bar", "area", "gauge", "heatmap", "logs"}
	for _, value := range valid {
		if !validPanelVisualization(value) {
			t.Errorf("validPanelVisualization(%q) = false", value)
		}
	}
	invalid := []string{"", "pie", "iframe", "unknown"}
	for _, value := range invalid {
		if validPanelVisualization(value) {
			t.Errorf("validPanelVisualization(%q) = true", value)
		}
	}
}

func TestValidPanelGridWidth(t *testing.T) {
	for _, value := range []int{4, 6, 8, 12} {
		if !validPanelGridWidth(value) {
			t.Errorf("validPanelGridWidth(%d) = false", value)
		}
	}
	for _, value := range []int{-1, 0, 1, 2, 3, 5, 7, 9, 10, 11, 13, 24} {
		if validPanelGridWidth(value) {
			t.Errorf("validPanelGridWidth(%d) = true", value)
		}
	}
}

func TestValidDashboardTimeRange(t *testing.T) {
	for _, value := range []string{"panel", "1h", "6h", "24h", "7d", "30d"} {
		if !validDashboardTimeRange(value) {
			t.Errorf("validDashboardTimeRange(%q) = false", value)
		}
	}
	for _, value := range []string{"", "15m", "90d", "all", "invalid"} {
		if validDashboardTimeRange(value) {
			t.Errorf("validDashboardTimeRange(%q) = true", value)
		}
	}
}
