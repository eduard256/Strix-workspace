package probe

import "probe/pkg/probe"

func init() {
	RegisterDetector(func(r *probe.Response) string {
		if r.Probes.MDNS != nil && !r.Probes.MDNS.Paired {
			if r.Probes.MDNS.Category == "camera" || r.Probes.MDNS.Category == "doorbell" {
				return "homekit"
			}
		}
		return ""
	})
}
