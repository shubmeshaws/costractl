package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ValidateResponse struct {
	Valid                     bool   `json:"valid"`
	OrganizationID            string `json:"organizationId"`
	OrganizationName          string `json:"organizationName"`
	TokenPrefix               string `json:"tokenPrefix"`
	PermissionsDocURL         string `json:"permissionsDocUrl"`
	InstalledTiersAvailable   struct {
		CostMonitoring      bool `json:"costMonitoring"`
		ClusterOptimization bool `json:"clusterOptimization"`
		WorkloadAutoscaler  bool `json:"workloadAutoscaler"`
	} `json:"installedTiersAvailable"`
}

type ConnectBundle struct {
	ClusterID         string   `json:"clusterId"`
	ClusterName       string   `json:"clusterName"`
	AgentToken        string   `json:"agentToken"`
	IngestURL         string   `json:"ingestUrl"`
	AgentImage        string   `json:"agentImage"`
	Namespace         string   `json:"namespace"`
	EnabledTiers      []string `json:"enabledTiers"`
	TiersInstalled    []string `json:"tiersInstalled"`
	TiersRequested    []string `json:"tiersRequested"`
	TiersPending      []string `json:"tiersPending"`
	PermissionsDocURL string   `json:"permissionsDocUrl"`
}

type ConnectOptions struct {
	ClusterName         string
	KubeContext         string
	CostMonitoring      bool
	ClusterOptimization bool
	WorkloadAutoscaler  bool
}

type Client struct {
	baseURL string
	token   string
	orgID   string
	http    *http.Client
}

func New(baseURL, token, orgID string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		orgID:   orgID,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) authRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Organization-Id", c.orgID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func (c *Client) ValidateToken(ctx context.Context) (*ValidateResponse, error) {
	resp, err := c.authRequest(ctx, http.MethodGet, "/api/collector/validate-token?organizationId="+c.orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("validate token: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("invalid or expired API token — generate a new token in Costra Settings → Clusters")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("validate failed (%d): %s", resp.StatusCode, string(b))
	}

	var out ValidateResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if !out.Valid {
		return nil, fmt.Errorf("API token validation failed")
	}
	return &out, nil
}

func (c *Client) Connect(ctx context.Context, opts ConnectOptions) (*ConnectBundle, error) {
	body := map[string]interface{}{
		"clusterName":         opts.ClusterName,
		"kubeContext":         opts.KubeContext,
		"costMonitoring":      opts.CostMonitoring,
		"clusterOptimization": opts.ClusterOptimization,
		"workloadAutoscaler":  opts.WorkloadAutoscaler,
	}
	raw, _ := json.Marshal(body)

	resp, err := c.authRequest(ctx, http.MethodPost, "/api/collector/connect", raw)
	if err != nil {
		return nil, fmt.Errorf("connect request failed: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("connect failed (%d): %s", resp.StatusCode, string(b))
	}

	var envelope struct {
		Success bool          `json:"success"`
		Data    ConnectBundle `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type DisconnectResult struct {
	ClusterID   string `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
}

func (c *Client) Disconnect(ctx context.Context, clusterName string) (*DisconnectResult, error) {
	body := map[string]interface{}{
		"clusterName": clusterName,
	}
	raw, _ := json.Marshal(body)

	resp, err := c.authRequest(ctx, http.MethodPost, "/api/collector/disconnect", raw)
	if err != nil {
		return nil, fmt.Errorf("disconnect request failed: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("cluster \"%s\" not found in Costra — it may already be disconnected", clusterName)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("disconnect failed (%d): %s", resp.StatusCode, string(b))
	}

	var envelope struct {
		Success bool             `json:"success"`
		Data    DisconnectResult `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}
