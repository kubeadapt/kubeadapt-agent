package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newAWSServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			if r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds") == "" {
				http.Error(w, "missing ttl header", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, "test-token-abc123")

		case r.Method == http.MethodGet && r.URL.Path == "/latest/dynamic/instance-identity/document":
			if r.Header.Get("X-aws-ec2-metadata-token") != "test-token-abc123" {
				http.Error(w, "bad token", http.StatusUnauthorized)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{
				"accountId":        "123456789012",
				"region":           "us-east-1",
				"availabilityZone": "us-east-1a",
				"instanceType":     "m5.xlarge",
				"instanceId":       "i-0abcdef1234567890",
			})

		default:
			http.NotFound(w, r)
		}
	}))
}

func TestDetectAWS_Success(t *testing.T) {
	srv := newAWSServer(t)
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	md, err := detectAWSWithBase(context.Background(), client, srv.URL)
	if err != nil {
		t.Fatalf("detectAWS returned error: %v", err)
	}

	if md.Provider != "aws" {
		t.Errorf("Provider = %q, want %q", md.Provider, "aws")
	}
	if md.AccountID != "123456789012" {
		t.Errorf("AccountID = %q, want %q", md.AccountID, "123456789012")
	}
	if md.Region != "us-east-1" {
		t.Errorf("Region = %q, want %q", md.Region, "us-east-1")
	}
	if md.Zone != "us-east-1a" {
		t.Errorf("Zone = %q, want %q", md.Zone, "us-east-1a")
	}
	if md.InstanceType != "m5.xlarge" {
		t.Errorf("InstanceType = %q, want %q", md.InstanceType, "m5.xlarge")
	}
	if md.InstanceID != "i-0abcdef1234567890" {
		t.Errorf("InstanceID = %q, want %q", md.InstanceID, "i-0abcdef1234567890")
	}
}

func TestDetectAWS_TokenFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/latest/api/token" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	_, err := detectAWSWithBase(context.Background(), client, srv.URL)
	if err == nil {
		t.Fatal("expected error when IMDS token returns 403")
	}
}

func TestDetectAWS_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	_, err := detectAWSWithBase(ctx, client, srv.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func newGCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "missing Metadata-Flavor header", http.StatusForbidden)
			return
		}

		switch r.URL.Path {
		case "/computeMetadata/v1/project/project-id":
			fmt.Fprint(w, "my-gcp-project-123")
		case "/computeMetadata/v1/instance/zone":
			fmt.Fprint(w, "projects/123456789/zones/us-central1-a")
		case "/computeMetadata/v1/instance/machine-type":
			fmt.Fprint(w, "projects/123456789/machineTypes/n1-standard-4")
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestDetectGCP_Success(t *testing.T) {
	srv := newGCPServer(t)
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	md, err := detectGCPWithBase(context.Background(), client, srv.URL+"/computeMetadata/v1")
	if err != nil {
		t.Fatalf("detectGCP returned error: %v", err)
	}

	if md.Provider != "gcp" {
		t.Errorf("Provider = %q, want %q", md.Provider, "gcp")
	}
	if md.AccountID != "my-gcp-project-123" {
		t.Errorf("AccountID = %q, want %q", md.AccountID, "my-gcp-project-123")
	}
	if md.Region != "us-central1" {
		t.Errorf("Region = %q, want %q", md.Region, "us-central1")
	}
	if md.Zone != "us-central1-a" {
		t.Errorf("Zone = %q, want %q", md.Zone, "us-central1-a")
	}
	if md.InstanceType != "n1-standard-4" {
		t.Errorf("InstanceType = %q, want %q", md.InstanceType, "n1-standard-4")
	}
}

func TestDetectGCP_MissingHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	_, err := detectGCPWithBase(context.Background(), client, srv.URL+"/computeMetadata/v1")
	if err == nil {
		t.Fatal("expected error when Metadata-Flavor header is missing and server returns 403")
	}
}

func newAzureServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata") != "true" {
			http.Error(w, "missing Metadata header", http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/metadata/instance" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"compute": map[string]string{
				"subscriptionId": "sub-abc-123",
				"location":       "eastus",
				"vmSize":         "Standard_D4s_v3",
				"vmId":           "vm-12345678",
			},
		})
	}))
}

func TestDetectAzure_Success(t *testing.T) {
	srv := newAzureServer(t)
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	md, err := detectAzureWithBase(context.Background(), client, srv.URL)
	if err != nil {
		t.Fatalf("detectAzure returned error: %v", err)
	}

	if md.Provider != "azure" {
		t.Errorf("Provider = %q, want %q", md.Provider, "azure")
	}
	if md.AccountID != "sub-abc-123" {
		t.Errorf("AccountID = %q, want %q", md.AccountID, "sub-abc-123")
	}
	if md.Region != "eastus" {
		t.Errorf("Region = %q, want %q", md.Region, "eastus")
	}
	if md.InstanceType != "Standard_D4s_v3" {
		t.Errorf("InstanceType = %q, want %q", md.InstanceType, "Standard_D4s_v3")
	}
	if md.InstanceID != "vm-12345678" {
		t.Errorf("InstanceID = %q, want %q", md.InstanceID, "vm-12345678")
	}
}

func TestDetectCloudMetadata_FirstWins(t *testing.T) {
	awsSrv := newAWSServer(t)
	defer awsSrv.Close()

	gcpCalled := false
	azureCalled := false
	gcpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gcpCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer gcpSrv.Close()
	azureSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer azureSrv.Close()

	md := detectCloudMetadataWithBases(context.Background(), 2*time.Second, awsSrv.URL, gcpSrv.URL+"/computeMetadata/v1", azureSrv.URL)

	if md.Provider != "aws" {
		t.Errorf("Provider = %q, want %q", md.Provider, "aws")
	}
	if gcpCalled {
		t.Error("GCP server was called, but AWS should have won")
	}
	if azureCalled {
		t.Error("Azure server was called, but AWS should have won")
	}
}

func TestDetectCloudMetadata_Fallthrough(t *testing.T) {
	awsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer awsSrv.Close()

	gcpSrv := newGCPServer(t)
	defer gcpSrv.Close()

	azureCalled := false
	azureSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer azureSrv.Close()

	md := detectCloudMetadataWithBases(context.Background(), 2*time.Second, awsSrv.URL, gcpSrv.URL+"/computeMetadata/v1", azureSrv.URL)

	if md.Provider != "gcp" {
		t.Errorf("Provider = %q, want %q", md.Provider, "gcp")
	}
	if azureCalled {
		t.Error("Azure server was called, but GCP should have won")
	}
}

func TestDetectCloudMetadata_AllFail(t *testing.T) {
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer failSrv.Close()

	md := detectCloudMetadataWithBases(context.Background(), 2*time.Second, failSrv.URL, failSrv.URL+"/computeMetadata/v1", failSrv.URL)

	if md.Provider != "" {
		t.Errorf("Provider = %q, want empty", md.Provider)
	}
	if md.AccountID != "" {
		t.Errorf("AccountID = %q, want empty", md.AccountID)
	}
	if md.Region != "" {
		t.Errorf("Region = %q, want empty", md.Region)
	}
}

// --- Test helpers that call unexported functions with custom base URLs ---

func detectAWSWithBase(ctx context.Context, client *http.Client, baseURL string) (CloudMetadata, error) {
	return detectAWSURL(ctx, client, baseURL)
}

func detectGCPWithBase(ctx context.Context, client *http.Client, baseURL string) (CloudMetadata, error) {
	return detectGCPURL(ctx, client, baseURL)
}

func detectAzureWithBase(ctx context.Context, client *http.Client, baseURL string) (CloudMetadata, error) {
	return detectAzureURL(ctx, client, baseURL)
}

func detectCloudMetadataWithBases(ctx context.Context, timeout time.Duration, awsBase, gcpBase, azureBase string) CloudMetadata {
	client := &http.Client{Timeout: timeout}

	type detectFunc struct {
		name string
		fn   func(context.Context, *http.Client) (CloudMetadata, error)
	}

	providers := []detectFunc{
		{"aws", func(ctx context.Context, c *http.Client) (CloudMetadata, error) {
			return detectAWSURL(ctx, c, awsBase)
		}},
		{"gcp", func(ctx context.Context, c *http.Client) (CloudMetadata, error) {
			return detectGCPURL(ctx, c, gcpBase)
		}},
		{"azure", func(ctx context.Context, c *http.Client) (CloudMetadata, error) {
			return detectAzureURL(ctx, c, azureBase)
		}},
	}

	for _, p := range providers {
		md, err := p.fn(ctx, client)
		if err != nil {
			continue
		}
		return md
	}

	return CloudMetadata{}
}
