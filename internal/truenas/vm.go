package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ahmd-soliman/naswarden/internal/vm"
)

type rawVM struct {
	ID                    int           `json:"id"`
	Name                  string        `json:"name"`
	Description           string        `json:"description"`
	VCPUs                 int           `json:"vcpus"`   // sockets
	Cores                 int           `json:"cores"`   // cores per socket
	Threads               int           `json:"threads"` // threads per core
	CPUMode               string        `json:"cpu_mode"`
	Memory                int64         `json:"memory"`     // in MiB
	MinMemory             int64         `json:"min_memory"` // in MiB
	Autostart             bool          `json:"autostart"`
	Bootloader            string        `json:"bootloader"`
	TrustedPlatformModule bool          `json:"trusted_platform_module"`
	HyperVEnlightenments  bool          `json:"hyperv_enlightenments"`
	Status                rawVMStatus   `json:"status"`
	Devices               []rawVMDevice `json:"devices"`
}

type rawVMStatus struct {
	State       string `json:"state"`        // "STOPPED", "RUNNING", "SUSPENDED"
	DomainState string `json:"domain_state"` // "SHUTOFF", "RUNNING"
	PID         *int   `json:"pid"`
}

type rawVMDevice struct {
	ID         int            `json:"id"`
	VM         int            `json:"vm"`
	Order      int            `json:"order"`
	Attributes map[string]any `json:"attributes"`
}

type pciChoice struct {
	Description    string `json:"description"`
	ControllerType string `json:"controller_type"`
	Capability     struct {
		Product string `json:"product"`
		Vendor  string `json:"vendor"`
	} `json:"capability"`
}

type rawZvol struct {
	Name    string   `json:"name"`
	VolSize property `json:"volsize"`
	Used    property `json:"used"`
}

type zvolInfo struct {
	volsize int64
	used    int64
}

// ListVMs queries TrueNAS SCALE native virtual machines over WebSocket,
// resolves backing ZFS zvol storage and PCI passthrough devices, and returns
// them as unified vm.Instance models with Manager="truenas".
func ListVMs(ctx context.Context, c *Client) ([]vm.Instance, error) {
	raw, err := c.Call(ctx, "vm.query", []any{})
	if err != nil {
		return nil, fmt.Errorf("vm.query: %w", err)
	}

	var rawVMs []rawVM
	if err := json.Unmarshal(raw, &rawVMs); err != nil {
		return nil, fmt.Errorf("vm.query: decode: %w", err)
	}

	if len(rawVMs) == 0 {
		return []vm.Instance{}, nil
	}

	// Best-effort: query volume datasets to resolve zvol used and total size
	zvolMap := make(map[string]zvolInfo)
	if zvolRaw, err := c.Call(ctx, "pool.dataset.query", []any{[]any{[]any{"type", "=", "VOLUME"}}}); err == nil {
		var zvols []rawZvol
		if err := json.Unmarshal(zvolRaw, &zvols); err == nil {
			for _, z := range zvols {
				var info zvolInfo
				if z.VolSize.Parsed != nil {
					info.volsize = *z.VolSize.Parsed
				}
				if z.Used.Parsed != nil {
					info.used = *z.Used.Parsed
				}
				zvolMap[z.Name] = info
			}
		}
	}

	// Best-effort: query PCI passthrough device choices to resolve human-readable GPU/audio labels
	pciMap := make(map[string]string)
	if pciRaw, err := c.Call(ctx, "vm.device.passthrough_device_choices", []any{}); err == nil {
		var choices map[string]pciChoice
		if err := json.Unmarshal(pciRaw, &choices); err == nil {
			for k, v := range choices {
				pciMap[k] = formatPCIDevice(v)
			}
		}
	}

	instances := make([]vm.Instance, 0, len(rawVMs))
	for _, raw := range rawVMs {
		inst := vm.Instance{
			Name:        raw.Name,
			Type:        "virtual-machine",
			Manager:     "truenas",
			IsVM:        true,
			IPv4:        []string{},
			Config:      make(map[string]string),
			Passthrough: []string{},
			AutoStart:   raw.Autostart,
		}

		// Status and StatusCode
		switch strings.ToUpper(raw.Status.State) {
		case "RUNNING":
			inst.Status = "Running"
			inst.StatusCode = 103
		case "STOPPED":
			inst.Status = "Stopped"
			inst.StatusCode = 102
		case "SUSPENDED", "PAUSED":
			inst.Status = "Frozen"
			inst.StatusCode = 105
		default:
			inst.Status = "Stopped"
			inst.StatusCode = 102
			if raw.Status.State != "" {
				inst.Status = strings.Title(strings.ToLower(raw.Status.State))
			}
		}

		// vCPU calculation: sockets * cores * threads
		cores := raw.VCPUs
		if raw.Cores > 0 {
			cores = raw.VCPUs * raw.Cores
		}
		if raw.Threads > 0 {
			cores *= raw.Threads
		}
		inst.CPUCores = cores

		// Memory: TrueNAS reports memory in MiB
		inst.MemTotal = raw.Memory * 1024 * 1024

		// Live memory if running: attempt vm.get_memory_usage
		if inst.StatusCode == 103 {
			if memRaw, err := c.Call(ctx, "vm.get_memory_usage", []any{raw.ID}); err == nil {
				var memUsage struct {
					Usage int64 `json:"usage"`
				}
				if err := json.Unmarshal(memRaw, &memUsage); err == nil && memUsage.Usage > 0 {
					inst.MemUsed = memUsage.Usage
				}
			}
		}

		// Inspect devices: DISK, NIC, DISPLAY, CDROM, PCI
		var cdromPaths []string
		for _, dev := range raw.Devices {
			dtype, _ := dev.Attributes["dtype"].(string)
			switch dtype {
			case "DISK":
				path, _ := dev.Attributes["path"].(string)
				if path != "" {
					zvolName := strings.TrimPrefix(path, "/dev/zvol/")
					if parts := strings.Split(zvolName, "/"); len(parts) > 0 && inst.DiskPool == "" {
						inst.DiskPool = parts[0]
					}
					if info, ok := zvolMap[zvolName]; ok {
						inst.DiskUsed += info.used
						inst.DiskTotal += info.volsize
					}
				}
			case "NIC":
				if mac, ok := dev.Attributes["mac"].(string); ok && mac != "" && inst.MAC == "" {
					inst.MAC = mac
				}
				if attach, ok := dev.Attributes["nic_attach"].(string); ok && attach != "" && inst.Bridge == "" {
					inst.Bridge = attach
				}
			case "DISPLAY":
				if p, ok := dev.Attributes["port"].(float64); ok && p > 0 {
					inst.DisplayPort = int(p)
				}
				if wp, ok := dev.Attributes["web_port"].(float64); ok && wp > 0 {
					inst.WebPort = int(wp)
				}
				if dispType, ok := dev.Attributes["type"].(string); ok && dispType != "" {
					inst.Config["display.type"] = dispType
				}
				if res, ok := dev.Attributes["resolution"].(string); ok && res != "" {
					inst.Config["display.resolution"] = res
				}
			case "CDROM":
				if p, ok := dev.Attributes["path"].(string); ok && p != "" {
					cdromPaths = append(cdromPaths, p)
				}
			case "PCI":
				if pptdev, ok := dev.Attributes["pptdev"].(string); ok && pptdev != "" {
					label := pptdev
					if resolved, ok := pciMap[pptdev]; ok && resolved != "" {
						label = resolved
					}
					inst.Passthrough = append(inst.Passthrough, label)
				}
			}
		}

		// Operating System detection
		inst.OS = detectOS(raw.Name, cdromPaths, raw.Description)

		// Config metadata
		if raw.Bootloader != "" {
			inst.Config["bootloader"] = raw.Bootloader
		}
		if raw.CPUMode != "" {
			inst.Config["cpu_mode"] = raw.CPUMode
		}
		if raw.TrustedPlatformModule {
			inst.Config["tpm"] = "enabled"
		}
		if raw.HyperVEnlightenments {
			inst.Config["hyperv_enlightenments"] = "enabled"
		}
		if inst.DisplayPort > 0 {
			inst.Config["display.port"] = fmt.Sprintf("%d", inst.DisplayPort)
			if inst.WebPort > 0 {
				inst.Config["display.web_port"] = fmt.Sprintf("%d", inst.WebPort)
			}
		}
		if len(inst.Passthrough) > 0 {
			inst.Config["passthrough"] = strings.Join(inst.Passthrough, ", ")
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

// formatPCIDevice formats a human-readable name for a PCI passthrough device.
// e.g. "NVIDIA GeForce GTX 1050 Ti" or "NVIDIA High Definition Audio Controller".
func formatPCIDevice(choice pciChoice) string {
	product := strings.TrimSpace(choice.Capability.Product)
	vendor := strings.TrimSpace(choice.Capability.Vendor)

	// Clean vendor (e.g. "NVIDIA Corporation" -> "NVIDIA", "Advanced Micro Devices, Inc. [AMD]" -> "AMD")
	if strings.Contains(vendor, "NVIDIA") {
		vendor = "NVIDIA"
	} else if strings.Contains(vendor, "AMD") {
		vendor = "AMD"
	} else if strings.Contains(vendor, "Intel") {
		vendor = "Intel"
	}

	// If product has bracketed model, e.g. "GP107 [GeForce GTX 1050 Ti]", extract inside brackets
	reBracket := regexp.MustCompile(`\[([^\]]+)\]`)
	if match := reBracket.FindStringSubmatch(product); len(match) > 1 {
		product = match[1]
	} else {
		// Strip chip codename prefix like "GP107GL High Definition Audio Controller"
		reChip := regexp.MustCompile(`^[A-Z0-9]{4,10}\s+`)
		product = reChip.ReplaceAllString(product, "")
	}

	if vendor != "" && !strings.Contains(strings.ToLower(product), strings.ToLower(vendor)) {
		return fmt.Sprintf("%s %s", vendor, product)
	}
	if product != "" {
		return product
	}
	if choice.Description != "" {
		return choice.Description
	}
	return "PCI Device"
}

// detectOS infers the guest operating system name from CDROM ISO paths, description, or VM name.
func detectOS(vmName string, cdromPaths []string, description string) string {
	for _, p := range cdromPaths {
		base := filepath.Base(p)
		baseLower := strings.ToLower(base)

		// Ignore driver ISOs like virtio-win-0.1.266.iso
		if strings.Contains(baseLower, "virtio") {
			continue
		}

		switch {
		case strings.Contains(baseLower, "win10") || strings.Contains(baseLower, "windows10") || strings.Contains(baseLower, "windows_10"):
			if strings.Contains(baseLower, "22h2") {
				return "Windows 10 (22H2)"
			}
			if strings.Contains(baseLower, "21h2") {
				return "Windows 10 (21H2)"
			}
			return "Windows 10"
		case strings.Contains(baseLower, "win11") || strings.Contains(baseLower, "windows11") || strings.Contains(baseLower, "windows_11"):
			if strings.Contains(baseLower, "24h2") {
				return "Windows 11 (24H2)"
			}
			if strings.Contains(baseLower, "23h2") {
				return "Windows 11 (23H2)"
			}
			return "Windows 11"
		case strings.Contains(baseLower, "winserver") || strings.Contains(baseLower, "windows_server"):
			return "Windows Server"
		case strings.Contains(baseLower, "ubuntu"):
			re := regexp.MustCompile(`ubuntu-?(\d+\.\d+)`)
			if m := re.FindStringSubmatch(baseLower); len(m) > 1 {
				return fmt.Sprintf("Ubuntu %s", m[1])
			}
			return "Ubuntu"
		case strings.Contains(baseLower, "debian"):
			re := regexp.MustCompile(`debian-?(\d+)`)
			if m := re.FindStringSubmatch(baseLower); len(m) > 1 {
				return fmt.Sprintf("Debian %s", m[1])
			}
			return "Debian"
		case strings.Contains(baseLower, "fedora"):
			return "Fedora"
		case strings.Contains(baseLower, "arch"):
			return "Arch Linux"
		}
	}

	if description != "" {
		return description
	}

	nameLower := strings.ToLower(vmName)
	switch {
	case strings.Contains(nameLower, "win"):
		return "Windows"
	case strings.Contains(nameLower, "ubuntu"):
		return "Ubuntu"
	case strings.Contains(nameLower, "debian"):
		return "Debian"
	default:
		return "Guest OS"
	}
}
