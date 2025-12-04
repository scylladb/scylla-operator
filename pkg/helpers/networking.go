package helpers

import (
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
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

func IsIPv6(ip net.IP) bool {
	return ip != nil && ip.To4() == nil
}

func IsIPv4(ip net.IP) bool {
	return ip != nil && ip.To4() != nil
}

// GetIPFamily figures out if an IP is IPv4 or IPv6
func GetIPFamily(ip net.IP) corev1.IPFamily {
	if IsIPv6(ip) {
		return corev1.IPv6Protocol
	}
	return corev1.IPv4Protocol
}

// GetPreferredIP picks the best IP from a list based on what you prefer.
// If you don't specify a preference, it just returns the first one.
func GetPreferredIP(ips []string, preferredFamily *corev1.IPFamily) (string, error) {
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses provided")
	}

	if preferredFamily == nil {
		return ips[0], nil
	}

	for _, ipStr := range ips {
		ip, err := ParseIP(ipStr)
		if err != nil {
			continue // Skip bad IPs
		}

		if GetIPFamily(ip) == *preferredFamily {
			return ipStr, nil
		}
	}

	return ips[0], nil
}

func GetPreferredPodIP(pod *corev1.Pod, preferredFamily *corev1.IPFamily) (string, error) {
	if pod == nil {
		return "", fmt.Errorf("pod is nil")
	}

	// Use PodIPs first as it's the canonical source and maintains proper order
	if len(pod.Status.PodIPs) > 0 {
		var ips []string
		seen := make(map[string]bool)

		for _, podIP := range pod.Status.PodIPs {
			if podIP.IP != "" && !seen[podIP.IP] {
				ips = append(ips, podIP.IP)
				seen[podIP.IP] = true
			}
		}

		if len(ips) > 0 {
			return GetPreferredIP(ips, preferredFamily)
		}
	}

	// Fallback to PodIP for older Kubernetes versions or edge cases
	if pod.Status.PodIP != "" {
		return pod.Status.PodIP, nil
	}

	return "", fmt.Errorf("no IP addresses found for pod %s/%s", pod.Namespace, pod.Name)
}

func GetPreferredServiceIP(svc *corev1.Service, preferredFamily *corev1.IPFamily) (string, error) {
	if svc == nil {
		return "", fmt.Errorf("service is nil")
	}

	if len(svc.Spec.ClusterIPs) == 0 || svc.Spec.ClusterIPs[0] == "None" {
		return "", fmt.Errorf("service has no cluster IPs")
	}

	// Filter out any invalid IPs and build a clean list
	var validIPs []string
	for _, ip := range svc.Spec.ClusterIPs {
		if ip != "" && ip != "None" {
			if parsedIP := net.ParseIP(ip); parsedIP != nil {
				validIPs = append(validIPs, ip)
			}
		}
	}

	if len(validIPs) == 0 {
		return "", fmt.Errorf("service has no valid cluster IPs")
	}

	// Use the existing GetPreferredIP helper which already handles the preference logic
	return GetPreferredIP(validIPs, preferredFamily)
}

// GetPreferredServiceIPFamily figures out what IP family a service prefers.
// If the service has IPFamilies set, it uses the first one. Otherwise it defaults to IPv4.
func GetPreferredServiceIPFamily(svc *corev1.Service) corev1.IPFamily {
	if svc == nil {
		return corev1.IPv4Protocol
	}

	if len(svc.Spec.IPFamilies) > 0 {
		return svc.Spec.IPFamilies[0]
	}

	return corev1.IPv4Protocol
}

// DetectEndpointsIPFamily detects the IP family from a list of endpoints.
// It returns the IP family of the first valid IP address found in the endpoints.
// If no valid IP addresses are found, it defaults to IPv4.
func DetectEndpointsIPFamily(endpoints []discoveryv1.Endpoint) corev1.IPFamily {
	for _, endpoint := range endpoints {
		if len(endpoint.Addresses) > 0 {
			parsedIP := net.ParseIP(endpoint.Addresses[0])
			if parsedIP != nil {
				return GetIPFamily(parsedIP)
			}
		}
	}

	// Default to IPv4 if no valid IP addresses found
	return corev1.IPv4Protocol
}
