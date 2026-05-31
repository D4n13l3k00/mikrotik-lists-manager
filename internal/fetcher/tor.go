package fetcher

import (
	"bufio"
	"net/http"
	"strings"
)

var torProvider = Provider{
	Name:  "Tor exit nodes",
	Slug:  "tor",
	Fetch: fetchTorExitNodes,
}

func fetchTorExitNodes(client *http.Client) ([]string, error) {
	body, err := get(client, "https://check.torproject.org/torbulkexitlist")
	if err != nil {
		return nil, err
	}
	var addrs []string
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if isIPv4CIDR(line) {
			addrs = append(addrs, line)
		}
	}
	return addrs, nil
}
