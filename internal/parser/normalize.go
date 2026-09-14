package parser

import (
	"fmt"
	"net"
	"strings"
)

// NormalizeAddr canonicalizes an IP address, CIDR, or domain name for uniform comparison.
// - Strips leading/trailing whitespace.
// - Non-IP addresses (domains, hostnames) are converted to lowercase.
// - Single IPv4 addresses are expanded to "/32" (e.g. "8.8.8.8" -> "8.8.8.8/32").
// - Single IPv6 addresses are expanded to "/128" in canonical form (e.g. "2001:db8::1" -> "2001:db8::1/128").
// - Valid CIDRs are normalized: the network portion has host bits zeroed out,
//   e.g. "192.168.1.5/24" becomes "192.168.1.0/24".
func NormalizeAddr(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") {
		ip, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			return strings.ToLower(s)
		}
		ones, bits := ipnet.Mask.Size()
		if (bits == 32 && ones == 32) || (bits == 128 && ones == 128) {
			return fmt.Sprintf("%s/%d", ip.String(), ones)
		}
		return fmt.Sprintf("%s/%d", ipnet.IP.String(), ones)
	}

	ip := net.ParseIP(s)
	if ip == nil {
		return strings.ToLower(s)
	}
	if ip.To4() != nil {
		return ip.String() + "/32"
	}
	return ip.String() + "/128"
}
