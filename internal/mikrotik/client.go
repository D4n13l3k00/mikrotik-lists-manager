package mikrotik

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/version"
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

// NewClient creates a direct REST API client (no proxy).
// host может быть: "192.168.1.1", "http://192.168.1.1", "https://192.168.1.1:8443".
// Если схема не указана — используется https.
// skipTLSVerify отключает проверку сертификата (актуально для самоподписанных).
func NewClient(host, user, pass string, skipTLSVerify bool) *Client {
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	host = strings.TrimRight(host, "/")

	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: skipTLSVerify}, //nolint:gosec
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		Proxy:               nil, // прямой доступ к роутеру
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

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}

	var (
		resp *http.Response
		err  error
	)
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = strings.NewReader(string(bodyBytes))
		}
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.user, c.pass)
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", version.UserAgent())
		resp, err = c.httpClient.Do(req)
		if err != nil {
			continue
		}
		// Non-5xx (success or client error) or final attempt: stop retrying.
		if resp.StatusCode < 500 || attempt == 2 {
			break
		}
		// Transient server error: drain body and retry.
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
		resp = nil
	}
	return resp, err
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// GetAllEntries returns all static entries across all address-lists.
func (c *Client) GetAllEntries(ctx context.Context) ([]AddressListEntry, error) {
	path := "/rest/ip/firewall/address-list?" + url.Values{"dynamic": {"false"}}.Encode()
	resp, err := c.do(ctx, "GET", path, nil)
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
func (c *Client) GetList(ctx context.Context, listName string) ([]AddressListEntry, error) {
	path := "/rest/ip/firewall/address-list?" + url.Values{
		"list":    {listName},
		"dynamic": {"false"},
	}.Encode()
	resp, err := c.do(ctx, "GET", path, nil)
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
// If the entry already exists on the router, the error is silently ignored.
func (c *Client) AddEntry(ctx context.Context, listName, address, comment string, disabled bool) error {
	payload := map[string]any{"list": listName, "address": address}
	if comment != "" {
		payload["comment"] = comment
	}
	if disabled {
		payload["disabled"] = true
	}
	b, _ := json.Marshal(payload)
	resp, err := c.do(ctx, "PUT", "/rest/ip/firewall/address-list", strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("add %s: %w", address, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "already have such entry") {
		return nil
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// UpdateEntry updates comment and disabled state of an existing entry by its MikroTik ID.
func (c *Client) UpdateEntry(ctx context.Context, id, comment string, disabled bool) error {
	b, _ := json.Marshal(map[string]any{"comment": comment, "disabled": disabled})
	// MikroTik IDs look like "*1A" — must NOT be percent-encoded.
	resp, err := c.do(ctx, "PATCH", "/rest/ip/firewall/address-list/"+id, strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("update %s: %w", id, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// SetDisabled sets disabled state on a single entry by ID.
func (c *Client) SetDisabled(ctx context.Context, id string, disabled bool) error {
	b, _ := json.Marshal(map[string]any{"disabled": disabled})
	resp, err := c.do(ctx, "PATCH", "/rest/ip/firewall/address-list/"+id, strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("set-disabled %s: %w", id, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// DeleteEntry removes an entry by its MikroTik ID.
func (c *Client) DeleteEntry(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/rest/ip/firewall/address-list/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete %s: %w", id, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// RouterInfo holds combined system info fetched from /rest/system/resource and /rest/system/routerboard.
type RouterInfo struct {
	BoardName       string
	Version         string
	Uptime          string
	Architecture    string
	CPU             string
	CPUCount        string
	TotalMemory     string
	FreeMemory      string
	// routerboard fields (empty for CHR/x86)
	Model           string
	Revision        string
	SerialNumber    string
	FirmwareType    string
	FactoryFirmware string
	CurrentFirmware string
	UpgradeFirmware string
}

type systemResource struct {
	BoardName    string `json:"board-name"`
	Version      string `json:"version"`
	Uptime       string `json:"uptime"`
	Architecture string `json:"architecture-name"`
	CPU          string `json:"cpu"`
	CPUCount     string `json:"cpu-count"`
	TotalMemory  string `json:"total-memory"`
	FreeMemory   string `json:"free-memory"`
}

type routerBoard struct {
	Model           string `json:"model"`
	Revision        string `json:"revision"`
	SerialNumber    string `json:"serial-number"`
	FirmwareType    string `json:"firmware-type"`
	FactoryFirmware string `json:"factory-firmware"`
	CurrentFirmware string `json:"current-firmware"`
	UpgradeFirmware string `json:"upgrade-firmware"`
}

// RenameEntry updates the list field for a single entry by ID.
func (c *Client) RenameEntry(ctx context.Context, id, newName string) error {
	b, _ := json.Marshal(map[string]any{"list": newName})
	resp, err := c.do(ctx, "PATCH", "/rest/ip/firewall/address-list/"+id, strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("rename %s: %w", id, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// RenameList renames an address-list by patching the list field of every entry in parallel.
// Returns the number of entries updated.
func (c *Client) RenameList(ctx context.Context, oldName, newName string, concurrency int, onProgress func(done, total int)) (int, error) {
	entries, err := c.GetList(ctx, oldName)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, fmt.Errorf("список %q не найден или пуст", oldName)
	}

	total := len(entries)
	if onProgress != nil {
		onProgress(0, total)
	}

	limit := concurrency
	if limit <= 0 {
		limit = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	var done atomic.Int64
	var progressMu sync.Mutex
	for _, e := range entries {
		if gctx.Err() != nil {
			break
		}
		entry := e
		g.Go(func() error {
			if err := c.RenameEntry(gctx, entry.ID, newName); err != nil {
				return fmt.Errorf("rename %s: %w", entry.Address, err)
			}
			d := int(done.Add(1))
			if onProgress != nil {
				progressMu.Lock()
				onProgress(d, total)
				progressMu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return int(done.Load()), err
	}
	return total, nil
}

// GetRouterInfo fetches system resource and routerboard info concurrently.
// RouterBoard endpoint failure is non-fatal (CHR/x86 devices have no routerboard).
func (c *Client) GetRouterInfo(ctx context.Context) (*RouterInfo, error) {
	var res systemResource
	var rb routerBoard
	var resErr error

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		resp, err := c.do(ctx, "GET", "/rest/system/resource", nil)
		if err != nil {
			resErr = err
			return
		}
		defer resp.Body.Close()
		if err := checkStatus(resp); err != nil {
			resErr = err
			return
		}
		json.NewDecoder(resp.Body).Decode(&res) //nolint:errcheck
	}()

	go func() {
		defer wg.Done()
		resp, err := c.do(ctx, "GET", "/rest/system/routerboard", nil)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if checkStatus(resp) != nil {
			return
		}
		json.NewDecoder(resp.Body).Decode(&rb) //nolint:errcheck
	}()

	wg.Wait()

	if resErr != nil {
		return nil, fmt.Errorf("system/resource: %w", resErr)
	}
	return &RouterInfo{
		BoardName:       res.BoardName,
		Version:         res.Version,
		Uptime:          res.Uptime,
		Architecture:    res.Architecture,
		CPU:             res.CPU,
		CPUCount:        res.CPUCount,
		TotalMemory:     res.TotalMemory,
		FreeMemory:      res.FreeMemory,
		Model:           rb.Model,
		Revision:        rb.Revision,
		SerialNumber:    rb.SerialNumber,
		FirmwareType:    rb.FirmwareType,
		FactoryFirmware: rb.FactoryFirmware,
		CurrentFirmware: rb.CurrentFirmware,
		UpgradeFirmware: rb.UpgradeFirmware,
	}, nil
}

// Execute runs an arbitrary RouterOS script or CLI command via REST API.
// It first attempts POST /rest/execute. If that returns 404 (endpoint unavailable),
// it falls back to creating, running, and deleting a temporary script via /rest/system/script.
func (c *Client) Execute(ctx context.Context, script string) error {
	payload, err := json.Marshal(map[string]string{"script": script})
	if err != nil {
		return fmt.Errorf("marshal execute payload: %w", err)
	}

	resp, err := c.do(ctx, "POST", "/rest/execute", strings.NewReader(string(payload)))
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		// If 404 Not Found, fall back to /rest/system/script
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			return checkStatus(resp)
		}
	}

	return c.executeViaSystemScript(ctx, script)
}

func (c *Client) executeViaSystemScript(ctx context.Context, script string) error {
	scriptName := fmt.Sprintf("mlm_tmp_%d", time.Now().UnixNano())
	createPayload, err := json.Marshal(map[string]string{
		"name":                     scriptName,
		"source":                   script,
		"dont-require-permissions": "yes",
	})
	if err != nil {
		return fmt.Errorf("marshal script: %w", err)
	}

	createResp, err := c.do(ctx, "POST", "/rest/system/script", strings.NewReader(string(createPayload)))
	if err != nil {
		return fmt.Errorf("create script: %w", err)
	}
	defer createResp.Body.Close()
	if err := checkStatus(createResp); err != nil {
		return fmt.Errorf("create script: %w", err)
	}

	var created struct {
		ID string `json:".id"`
	}
	_ = json.NewDecoder(createResp.Body).Decode(&created)

	defer func() {
		target := scriptName
		if created.ID != "" {
			target = created.ID
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		delResp, delErr := c.do(cleanupCtx, "DELETE", "/rest/system/script/"+target, nil)
		if delErr == nil {
			delResp.Body.Close()
		}
	}()

	runPayload, err := json.Marshal(map[string]string{
		"number": scriptName,
	})
	if err != nil {
		return fmt.Errorf("marshal run script: %w", err)
	}

	runResp, err := c.do(ctx, "POST", "/rest/system/script/run", strings.NewReader(string(runPayload)))
	if err != nil {
		return fmt.Errorf("run script: %w", err)
	}
	defer runResp.Body.Close()
	return checkStatus(runResp)
}

