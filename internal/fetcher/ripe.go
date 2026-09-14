package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

type ripeStatResponse struct {
	Data struct {
		Prefixes []struct {
			Prefix string `json:"prefix"`
		} `json:"prefixes"`
	} `json:"data"`
}

// MakeASNProvider creates an ad-hoc Provider for a single ASN via RIPE STAT.
// Accepts "AS12345" or bare "12345" — normalizes to uppercase "AS…" form.
func MakeASNProvider(asn string) Provider {
	asn = strings.ToUpper(strings.TrimSpace(asn))
	if !strings.HasPrefix(asn, "AS") {
		asn = "AS" + asn
	}
	a := asn
	return Provider{
		Name:  a,
		Slug:  strings.ToLower(a),
		Fetch: func(c *http.Client) ([]string, error) { return fetchRIPEPrefixes(c, a) },
	}
}

func fetchSingleASN(client *http.Client, asn string) ([]string, error) {
	body, err := get(client, "https://stat.ripe.net/data/announced-prefixes/data.json?resource="+asn)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", asn, err)
	}
	var resp ripeStatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%s: parse JSON: %w", asn, err)
	}
	var prefixes []string
	for _, p := range resp.Data.Prefixes {
		if isIPv4CIDR(p.Prefix) {
			prefixes = append(prefixes, p.Prefix)
		}
	}
	return prefixes, nil
}

// fetchRIPEPrefixes queries RIPE STAT announced-prefixes for the given ASNs
// and returns deduplicated IPv4 CIDRs.
func fetchRIPEPrefixes(client *http.Client, asns ...string) ([]string, error) {
	if len(asns) == 0 {
		return nil, nil
	}
	if len(asns) == 1 {
		return fetchSingleASN(client, asns[0])
	}

	var (
		mu    sync.Mutex
		seen  = make(map[string]struct{})
		cidrs []string
	)

	g := new(errgroup.Group)
	g.SetLimit(3)

	for _, asn := range asns {
		a := asn
		g.Go(func() error {
			prefixes, err := fetchSingleASN(client, a)
			if err != nil {
				return err
			}
			mu.Lock()
			for _, p := range prefixes {
				if _, dup := seen[p]; !dup {
					seen[p] = struct{}{}
					cidrs = append(cidrs, p)
				}
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return cidrs, nil
}
