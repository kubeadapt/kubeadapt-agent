package gpu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const scrapeTimeout = 5 * time.Second

// scrapeEndpoint fetches raw Prometheus metrics text from a dcgm-exporter endpoint.
// The endpoint should be a base URL (e.g., "http://10.0.0.5:9400"); "/metrics" is appended.
func scrapeEndpoint(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	url := strings.TrimRight(endpoint, "/") + "/metrics"

	ctx, cancel := context.WithTimeout(ctx, scrapeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scraping %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}

	return body, nil
}
