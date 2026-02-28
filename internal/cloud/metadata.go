package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// CloudMetadata holds cloud provider identity information detected from instance metadata services.
type CloudMetadata struct {
	AccountID    string
	Region       string
	Zone         string
	InstanceType string
	InstanceID   string
	Provider     string // "aws", "gcp", "azure", ""
}

const (
	awsIMDSBase     = "http://169.254.169.254"
	gcpMetadataBase = "http://metadata.google.internal/computeMetadata/v1"
	azureIMDSBase   = "http://169.254.169.254"
)

// DetectCloudMetadata probes AWS, GCP, and Azure IMDS endpoints to identify the current cloud provider.
func DetectCloudMetadata(ctx context.Context, timeout time.Duration) CloudMetadata {
	client := &http.Client{Timeout: timeout}

	type detectFunc struct {
		name string
		fn   func(context.Context, *http.Client) (CloudMetadata, error)
	}

	providers := []detectFunc{
		{"aws", detectAWS},
		{"gcp", detectGCP},
		{"azure", detectAzure},
	}

	for _, p := range providers {
		md, err := p.fn(ctx, client)
		if err != nil {
			slog.Debug("cloud metadata detection failed", "provider", p.name, "error", err)
			continue
		}
		slog.Debug("cloud metadata detected", "provider", p.name, "account", md.AccountID, "region", md.Region)
		return md
	}

	return CloudMetadata{}
}

func detectAWS(ctx context.Context, client *http.Client) (CloudMetadata, error) {
	return detectAWSURL(ctx, client, awsIMDSBase)
}

func detectAWSURL(ctx context.Context, client *http.Client, base string) (CloudMetadata, error) {
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPut, base+"/latest/api/token", nil)
	if err != nil {
		return CloudMetadata{}, err
	}
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return CloudMetadata{}, err
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		return CloudMetadata{}, fmt.Errorf("IMDS token request returned %d", tokenResp.StatusCode)
	}

	tokenBytes, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		return CloudMetadata{}, err
	}
	token := string(tokenBytes)

	docReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/latest/dynamic/instance-identity/document", nil)
	if err != nil {
		return CloudMetadata{}, err
	}
	docReq.Header.Set("X-aws-ec2-metadata-token", token)

	docResp, err := client.Do(docReq)
	if err != nil {
		return CloudMetadata{}, err
	}
	defer docResp.Body.Close()

	if docResp.StatusCode != http.StatusOK {
		return CloudMetadata{}, fmt.Errorf("IMDS identity document returned %d", docResp.StatusCode)
	}

	var doc struct {
		AccountID        string `json:"accountId"`
		Region           string `json:"region"`
		AvailabilityZone string `json:"availabilityZone"`
		InstanceType     string `json:"instanceType"`
		InstanceID       string `json:"instanceId"`
	}
	if err := json.NewDecoder(docResp.Body).Decode(&doc); err != nil {
		return CloudMetadata{}, err
	}

	return CloudMetadata{
		AccountID:    doc.AccountID,
		Region:       doc.Region,
		Zone:         doc.AvailabilityZone,
		InstanceType: doc.InstanceType,
		InstanceID:   doc.InstanceID,
		Provider:     "aws",
	}, nil
}

func detectGCP(ctx context.Context, client *http.Client) (CloudMetadata, error) {
	return detectGCPURL(ctx, client, gcpMetadataBase)
}

func detectGCPURL(ctx context.Context, client *http.Client, base string) (CloudMetadata, error) {
	get := func(path string) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Metadata-Flavor", "Google")

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("GCP metadata %s returned %d", path, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	projectID, err := get("/project/project-id")
	if err != nil {
		return CloudMetadata{}, err
	}

	zonePath, err := get("/instance/zone")
	if err != nil {
		return CloudMetadata{}, err
	}
	zone := zonePath
	if idx := strings.LastIndex(zonePath, "/"); idx >= 0 {
		zone = zonePath[idx+1:]
	}
	region := zone
	if idx := strings.LastIndex(zone, "-"); idx > 0 {
		region = zone[:idx]
	}

	machineTypePath, err := get("/instance/machine-type")
	if err != nil {
		return CloudMetadata{}, err
	}
	machineType := machineTypePath
	if idx := strings.LastIndex(machineTypePath, "/"); idx >= 0 {
		machineType = machineTypePath[idx+1:]
	}

	return CloudMetadata{
		AccountID:    projectID,
		Region:       region,
		Zone:         zone,
		InstanceType: machineType,
		Provider:     "gcp",
	}, nil
}

func detectAzure(ctx context.Context, client *http.Client) (CloudMetadata, error) {
	return detectAzureURL(ctx, client, azureIMDSBase)
}

func detectAzureURL(ctx context.Context, client *http.Client, base string) (CloudMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/metadata/instance?api-version=2021-02-01", nil)
	if err != nil {
		return CloudMetadata{}, err
	}
	req.Header.Set("Metadata", "true")

	resp, err := client.Do(req)
	if err != nil {
		return CloudMetadata{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CloudMetadata{}, fmt.Errorf("azure IMDS returned %d", resp.StatusCode)
	}

	var doc struct {
		Compute struct {
			SubscriptionID string `json:"subscriptionId"`
			Location       string `json:"location"`
			VMSize         string `json:"vmSize"`
			VMID           string `json:"vmId"`
		} `json:"compute"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return CloudMetadata{}, err
	}

	return CloudMetadata{
		AccountID:    doc.Compute.SubscriptionID,
		Region:       doc.Compute.Location,
		InstanceType: doc.Compute.VMSize,
		InstanceID:   doc.Compute.VMID,
		Provider:     "azure",
	}, nil
}
