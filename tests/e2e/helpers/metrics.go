package helpers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ScrapeMetrics fetches the raw Prometheus metrics text from the agent's /metrics endpoint.
func ScrapeMetrics(baseURL string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/metrics")
	if err != nil {
		return "", fmt.Errorf("GET /metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /metrics returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read metrics body: %w", err)
	}

	return string(body), nil
}

// AssertMetricExists checks that a metric name appears in the scraped metrics text.
func AssertMetricExists(t *testing.T, metricsText, metricName string) {
	t.Helper()
	if !strings.Contains(metricsText, metricName) {
		t.Errorf("metric %q not found in /metrics output", metricName)
	}
}

// AssertMetricWithLabel checks that a metric with a specific label value exists.
func AssertMetricWithLabel(t *testing.T, metricsText, metricName, labelKey, labelValue string) {
	t.Helper()
	expected := fmt.Sprintf(`%s="%s"`, labelKey, labelValue)
	found := false

	for _, line := range strings.Split(metricsText, "\n") {
		if strings.HasPrefix(line, metricName) && strings.Contains(line, expected) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("metric %s{%s} not found in /metrics output", metricName, expected)
	}
}

// GetMetricValue extracts the float64 value of a specific metric line.
// Returns 0 and false if the metric is not found.
func GetMetricValue(metricsText, metricName string) (float64, bool) {
	for _, line := range strings.Split(metricsText, "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, metricName) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var val float64
				if _, err := fmt.Sscanf(parts[len(parts)-1], "%f", &val); err == nil {
					return val, true
				}
			}
		}
	}
	return 0, false
}

// WaitForMetricExists polls /metrics until a metric appears.
func WaitForMetricExists(t *testing.T, baseURL, metricName string, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for metric %q", metricName)
			}

			text, err := ScrapeMetrics(baseURL)
			if err != nil {
				t.Logf("  scrape error: %v", err)
				continue
			}

			if strings.Contains(text, metricName) {
				t.Logf("✓ Metric %s found", metricName)
				return nil
			}

			t.Logf("  metric %s not found yet", metricName)
		}
	}
}
