package probe

import (
	"bufio"
	"os"
	"strings"
)

// LookupARP reads /proc/net/arp to find MAC address for ip.
// Linux only. Returns empty string if not found.
func LookupARP(ip string) string {
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan() // skip header

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		// fields[0] = IP, fields[3] = HW address
		if fields[0] == ip {
			mac := fields[3]
			if mac == "00:00:00:00:00:00" {
				return ""
			}
			return strings.ToUpper(mac)
		}
	}

	return ""
}
