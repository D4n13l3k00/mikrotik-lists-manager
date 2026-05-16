package mikrotik

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AddressListEntry is a single record from MikroTik REST API.
type AddressListEntry struct {
	ID       string     `json:".id"`
	Address  string     `json:"address"`
	List     string     `json:"list"`
	Comment  string     `json:"comment"`
	Disabled BoolString `json:"disabled"`
}

// BoolString unmarshals both JSON bool and "true"/"false" string from MikroTik.
type BoolString bool

func (b *BoolString) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	*b = BoolString(s == "true")
	return nil
}

func (b BoolString) Bool() bool { return bool(b) }

// Client talks to MikroTik REST API (RouterOS 7+).
type Client struct {
	baseURL    string
	httpClient *http.Client
	user       string
	pass       string
}

// NewClient creates a REST API client.
// host может быть: "192.168.1.1", "http://192.168.1.1", "https://192.168.1.1:8443".
// Если схема не указана — используется https.
// skipTLSVerify отключает проверку сертификата (актуально для самоподписанных).
func NewClient(host, user, pass string, skipTLSVerify bool) *Client {
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	host = strings.TrimRight(host, "/")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify}, //nolint:gosec
	}
	return &Client{
		baseURL: host,
		user:    user,
		pass:    pass,
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.pass)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return c.httpClient.Do(req)
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// GetAllEntries returns all static entries across all address-lists.
func (c *Client) GetAllEntries() ([]AddressListEntry, error) {
	path := "/rest/ip/firewall/address-list?" + url.Values{"dynamic": {"false"}}.Encode()
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("GET address-list: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var entries []AddressListEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return entries, nil
}

// GetList returns all static (dynamic=false) entries for the given address-list name.
func (c *Client) GetList(listName string) ([]AddressListEntry, error) {
	path := "/rest/ip/firewall/address-list?" + url.Values{
		"list":    {listName},
		"dynamic": {"false"},
	}.Encode()
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("GET address-list: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var entries []AddressListEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return entries, nil
}

// AddEntry adds a new entry to the address list.
func (c *Client) AddEntry(listName, address, comment string, disabled bool) error {
	payload := map[string]any{"list": listName, "address": address}
	if comment != "" {
		payload["comment"] = comment
	}
	if disabled {
		payload["disabled"] = true
	}
	b, _ := json.Marshal(payload)
	resp, err := c.do("PUT", "/rest/ip/firewall/address-list", strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("add %s: %w", address, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// UpdateEntry updates comment and disabled state of an existing entry by its MikroTik ID.
func (c *Client) UpdateEntry(id, comment string, disabled bool) error {
	b, _ := json.Marshal(map[string]any{"comment": comment, "disabled": disabled})
	// MikroTik IDs look like "*1A" — must NOT be percent-encoded.
	resp, err := c.do("PATCH", "/rest/ip/firewall/address-list/"+id, strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("update %s: %w", id, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// SetDisabled sets disabled state on a single entry by ID.
func (c *Client) SetDisabled(id string, disabled bool) error {
	b, _ := json.Marshal(map[string]any{"disabled": disabled})
	resp, err := c.do("PATCH", "/rest/ip/firewall/address-list/"+id, strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("set-disabled %s: %w", id, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// DeleteEntry removes an entry by its MikroTik ID.
func (c *Client) DeleteEntry(id string) error {
	resp, err := c.do("DELETE", "/rest/ip/firewall/address-list/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete %s: %w", id, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}
