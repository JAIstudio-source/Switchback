package server

import (
	"net"
	"strings"
)

// GetLocalIPs returns all non-loopback IPv4 addresses on the host.
func GetLocalIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{"127.0.0.1"}
	}

	for _, iface := range ifaces {
		// Ignore interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // Skip IPv6 for simple mobile pairing
			}
			// Prefer private IPv4 ranges (192.168.x.x, 10.x.x.x, 172.16-31.x.x)
			ipStr := ip.String()
			if strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") || strings.HasPrefix(ipStr, "172.") {
				ips = append([]string{ipStr}, ips...) // Prepend preferred
			} else {
				ips = append(ips, ipStr)
			}
		}
	}

	if len(ips) == 0 {
		ips = append(ips, "127.0.0.1")
	}
	return ips
}

// GetPrimaryLANIP returns the most likely local Wi-Fi / Ethernet LAN IP.
func GetPrimaryLANIP() string {
	ips := GetLocalIPs()
	if len(ips) > 0 {
		return ips[0]
	}
	return "127.0.0.1"
}
