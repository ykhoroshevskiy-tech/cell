package firecracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

type Client struct {
	socketPath string
	httpClient *http.Client
}

func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
	}
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

func (c *Client) request(method, path string, body io.Reader) error {
	req, err := http.NewRequest(method, "http://localhost"+path, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fc request %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fc %s %s (%d): %s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (c *Client) putJSON(path string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	verbose.V("fc PUT %s %s", path, strings.TrimSpace(string(data)))
	return c.request("PUT", path, strings.NewReader(string(data)))
}

func (c *Client) Configure(doc *models.VmConfigDocument) error {
	if err := c.putJSON("/boot-source", doc.BootSource); err != nil {
		return err
	}
	for _, drive := range doc.Drives {
		if err := c.putJSON("/drives/"+drive.DriveID, drive); err != nil {
			return err
		}
	}
	if err := c.putJSON("/machine-config", doc.MachineConfig); err != nil {
		return err
	}
	for _, iface := range doc.NetworkInterfaces {
		if err := c.putJSON("/network-interfaces/"+iface.IfaceID, iface); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) StartInstance() error {
	return c.putJSON("/actions", map[string]string{"action_type": "InstanceStart"})
}
