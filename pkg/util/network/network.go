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

func FindEthernetInterfaces() ([]net.Interface, error) {
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var ethInterfaces []net.Interface
	for _, iface := range hostInterfaces {
		// Skip loopback interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Tune only known prefixes
		for _, prefix := range knownInterfaceNamesPrefixes {
			if strings.HasPrefix(iface.Name, prefix) {
				ethInterfaces = append(ethInterfaces, iface)
				break
			}
		}
	}

	if len(ethInterfaces) != 0 {
		return ethInterfaces, nil
	}

	return nil, fmt.Errorf("local ethernet interface not found")
}
