package helpers

import (
	"fmt"
	"net"
)

func ParseIP(s string) (net.IP, error) {
	parsedIP := net.ParseIP(s)
	if parsedIP == nil {
		return nil, fmt.Errorf("can't parse IP %q", s)
	}

	return parsedIP, nil
}

func ParseIPs(ipStrings []string) ([]net.IP, error) {
	var ips []net.IP

	for _, s := range ipStrings {
		ip, err := ParseIP(s)
		if err != nil {
			return nil, err
		}
		ips = append(ips, ip)
	}

	return ips, nil
}

func NormalizeIPs(ips []net.IP) []net.IP {
	var res []net.IP

	for _, ip := range ips {
		res = append(res, ip.To16())
	}

	return res
}
