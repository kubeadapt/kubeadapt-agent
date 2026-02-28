package gpu

import (
	"context"
	"log/slog"
	"net/http"
)

// GPUMetricsAPI abstracts GPU metrics collection for testability.
type GPUMetricsAPI interface {
	ScrapeGPUMetrics(ctx context.Context, endpoints []string) ([]GPUDeviceMetrics, error)
}

// dcgmExporterClient implements GPUMetricsAPI by scraping dcgm-exporter endpoints.
type dcgmExporterClient struct {
	client *http.Client
}

// NewDCGMExporterClient creates a GPUMetricsAPI that scrapes dcgm-exporter HTTP endpoints.
func NewDCGMExporterClient(client *http.Client) GPUMetricsAPI {
	return &dcgmExporterClient{client: client}
}

func (c *dcgmExporterClient) ScrapeGPUMetrics(ctx context.Context, endpoints []string) ([]GPUDeviceMetrics, error) {
	var allMetrics []GPUDeviceMetrics

	for _, endpoint := range endpoints {
		body, err := scrapeEndpoint(ctx, c.client, endpoint)
		if err != nil {
			slog.Warn("failed to scrape dcgm-exporter",
				"endpoint", endpoint,
				"error", err,
			)
			continue
		}

		metrics, err := ParseDCGMMetrics(body)
		if err != nil {
			slog.Warn("failed to parse dcgm-exporter metrics",
				"endpoint", endpoint,
				"error", err,
			)
			continue
		}

		allMetrics = append(allMetrics, metrics...)
	}

	return allMetrics, nil
}
