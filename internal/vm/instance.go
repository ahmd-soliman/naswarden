package vm

import "sort"

// Instance is a virtual machine or container managed by Incus or TrueNAS.
type Instance struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`                   // "virtual-machine" or "container"
	Manager     string            `json:"manager"`                // "incus" | "truenas"
	Status      string            `json:"status"`                 // "Running", "Stopped", "Frozen", etc.
	StatusCode  int               `json:"status_code"`            // 103 (Running), 102 (Stopped)
	IsVM        bool              `json:"is_vm"`                  // true for KVM virtual-machine, false for LXC container
	OS          string            `json:"os"`                     // e.g. "Windows 10 (22H2)", "Ubuntu 24.04"
	Kernel      string            `json:"kernel,omitempty"`       // e.g. "6.8.0-139-generic"
	Arch        string            `json:"arch,omitempty"`         // e.g. "x86_64"
	CPUCores    int               `json:"cpu_cores"`              // Configured or detected vCPU count
	CPUPercent  float64           `json:"cpu_percent"`            // Live CPU % calculated between refresh intervals
	MemUsed     int64             `json:"mem_used"`               // Live memory bytes used
	MemTotal    int64             `json:"mem_total"`              // Memory limit or total bytes allocated
	DiskUsed    int64             `json:"disk_used"`              // Root disk zvol bytes used
	DiskTotal   int64             `json:"disk_total"`             // Root disk zvol capacity bytes
	DiskPool    string            `json:"disk_pool"`              // ZFS pool backing root disk (e.g. "fast")
	IPv4        []string          `json:"ipv4"`                   // Guest LAN IPv4 addresses
	MAC         string            `json:"mac,omitempty"`          // Primary network MAC address
	Bridge      string            `json:"bridge,omitempty"`       // Parent network bridge (e.g. "br0")
	StartedAt   string            `json:"started_at,omitempty"`   // RFC3339 timestamp
	AutoStart   bool              `json:"auto_start"`             // Whether instance is configured for auto-start on boot
	Config      map[string]string `json:"config,omitempty"`       // Config details (limits, bootloader, devices)
	DisplayPort int               `json:"display_port,omitempty"` // SPICE/VNC native port (e.g. 5901)
	WebPort     int               `json:"web_port,omitempty"`     // SPICE/VNC web port (e.g. 5902)
	Passthrough []string          `json:"passthrough,omitempty"`  // PCI passthrough devices (e.g. ["NVIDIA GeForce GTX 1050 Ti"])
}

// Sort orders instances the way every view wants them: KVM VMs before LXC
// containers, then by manager, then by name.
func Sort(instances []Instance) {
	sort.Slice(instances, func(i, j int) bool {
		a, b := instances[i], instances[j]
		if a.IsVM != b.IsVM {
			return a.IsVM
		}
		if a.Manager != b.Manager {
			return a.Manager < b.Manager
		}
		return a.Name < b.Name
	})
}
