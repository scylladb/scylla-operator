package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/pkg/errors"
)

func FindFirstNonLocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, errors.Wrap(err, "unable to discover interfaces")
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.To4(), nil
			}
		}
	}
	return nil, errors.New("no local ip found")
}

var knownInterfaceNamesPrefixes = []string{"eth", "eno", "ens", "enp"}

func FindEthernetInterface() (net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return net.Interface{}, err
	}

	// Return iface if it's one of the known.
	for _, knownPrefix := range knownInterfaceNamesPrefixes {
		for _, iface := range ifaces {
			if strings.HasPrefix(iface.Name, knownPrefix) {
				return iface, nil
			}
		}
	}

	// Fallback to non-loopback iface.
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback == 0 {
			return iface, nil
		}
	}

	return net.Interface{}, fmt.Errorf("local ethernet interface not found")
}
