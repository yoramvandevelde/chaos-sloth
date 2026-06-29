package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL     string
	tokenID     string
	tokenSecret string
	http        *http.Client
}

func New(baseURL, tokenID, tokenSecret string, insecureTLS bool) *Client {
	transport := &http.Transport{}
	if insecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		tokenID:     tokenID,
		tokenSecret: tokenSecret,
		http:        &http.Client{Transport: transport, Timeout: 30 * time.Second},
	}
}

func (c *Client) post(ctx context.Context, path string, params url.Values) error {
	target := c.baseURL + "/api2/json" + path

	var body io.Reader
	if len(params) > 0 {
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		var result struct {
			Errors interface{} `json:"errors"`
		}
		json.Unmarshal(raw, &result) //nolint:errcheck
		return fmt.Errorf("proxmox API %s %d: %s", path, resp.StatusCode, string(raw))
	}

	return nil
}

// HibernateVM saves VM state to disk and stops it ("Hibernate" in Proxmox UI).
func (c *Client) HibernateVM(ctx context.Context, node string, vmid int) error {
	return c.post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/suspend", node, vmid), url.Values{"todisk": {"1"}})
}

// PauseVM freezes the VM in RAM ("Pause" in Proxmox UI). Fast but VM still occupies host RAM.
func (c *Client) PauseVM(ctx context.Context, node string, vmid int) error {
	return c.post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/suspend", node, vmid), nil)
}

// ResumeVM resumes a hibernated or paused VM.
func (c *Client) ResumeVM(ctx context.Context, node string, vmid int) error {
	return c.post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/resume", node, vmid), nil)
}

func (c *Client) StopVM(ctx context.Context, node string, vmid int) error {
	return c.post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", node, vmid), nil)
}

func (c *Client) StartVM(ctx context.Context, node string, vmid int) error {
	return c.post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmid), nil)
}

func (c *Client) ResetVM(ctx context.Context, node string, vmid int) error {
	return c.post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/reset", node, vmid), nil)
}
