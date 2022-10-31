package helpers

import (
	"fmt"
	"net"
)

func ParseClusterIP(s string) (net.IP, error) {
	parsedIP := net.ParseIP(s)
	if parsedIP == nil {
		return nil, fmt.Errorf("can't parse ClusterIP %q", s)
	}

	return parsedIP, nil
}

func ParseClusterIPs(ipStrings []string) ([]net.IP, error) {
	var ips []net.IP

	for _, s := range ipStrings {
		ip, err := ParseClusterIP(s)
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
