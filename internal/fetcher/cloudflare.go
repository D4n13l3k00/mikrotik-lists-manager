package fetcher

import (
	"bufio"
	"net/http"
	"strings"
)

func fetchCloudflare(client *http.Client) ([]string, error) {
	body, err := get(client, "https://www.cloudflare.com/ips-v4")
	if err != nil {
		return nil, err
	}
	var cidrs []string
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); isIPv4CIDR(line) {
			cidrs = append(cidrs, line)
		}
	}
	return cidrs, nil
}
