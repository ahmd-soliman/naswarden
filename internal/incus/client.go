// Package incus monitors Incus virtual machines and LXC containers over
// its HTTPS REST API using mTLS client certificates.
package incus

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ahmd-soliman/naswarden/internal/vm"
)

// Instance is an alias to vm.Instance for backward compatibility.
type Instance = vm.Instance

type cpuSample struct {
	usageNs   int64
	timestamp time.Time
}

type Client struct {
	baseURL string
	http    *http.Client
	prevCPU map[string]cpuSample
	mu      sync.Mutex
}

// NewClient initializes an Incus API client with mTLS credentials.
// certData and keyData can be base64-encoded PEM, raw PEM text, or paths to files on disk.
func NewClient(rawURL, certData, keyData string, insecureTLS bool) (*Client, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("incus URL must not be empty")
	}
	baseURL := strings.TrimRight(rawURL, "/")

	certPEM, err := parsePEMOrFile(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to load incus client cert: %w", err)
	}

	keyPEM, err := parsePEMOrFile(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to load incus client key: %w", err)
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse incus x509 keypair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: insecureTLS,
	}

	transport := &http.Transport{
		TLSClientConfig:    tlsConfig,
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}

	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
		prevCPU: make(map[string]cpuSample),
	}, nil
}

func parsePEMOrFile(input string) ([]byte, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("empty certificate or key value")
	}

	if strings.HasPrefix(trimmed, "-----BEGIN") {
		return []byte(trimmed), nil
	}

	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && strings.Contains(string(decoded), "-----BEGIN") {
		return decoded, nil
	}

	if data, err := os.ReadFile(trimmed); err == nil {
		return data, nil
	}

	return nil, fmt.Errorf("value is neither PEM text, base64-encoded PEM, nor readable file path")
}

// Incus API response models for GET /1.0/instances?recursion=2
type apiResponse struct {
	Type       string        `json:"type"`
	Status     string        `json:"status"`
	StatusCode int           `json:"status_code"`
	Metadata   []rawInstance `json:"metadata"`
}

type rawInstance struct {
	Name       string                            `json:"name"`
	Status     string                            `json:"status"`
	StatusCode int                               `json:"status_code"`
	Type       string                            `json:"type"` // "virtual-machine" or "container"
	Config     map[string]string                 `json:"config"`
	Devices    map[string]map[string]interface{} `json:"devices"`
	State      *rawState                         `json:"state"`
}

type rawState struct {
	Status     string                `json:"status"`
	StatusCode int                   `json:"status_code"`
	StartedAt  string                `json:"started_at"`
	CPU        rawCPU                `json:"cpu"`
	Memory     rawMemory             `json:"memory"`
	Disk       map[string]rawDisk    `json:"disk"`
	Network    map[string]rawNetwork `json:"network"`
	OSInfo     rawOSInfo             `json:"os_info"`
	Pid        int                   `json:"pid"`
	Processes  int                   `json:"processes"`
}

type rawCPU struct {
	Usage int64 `json:"usage"`
}

type rawMemory struct {
	Usage     int64 `json:"usage"`
	Total     int64 `json:"total"`
	SwapUsage int64 `json:"swap_usage"`
}

type rawDisk struct {
	Usage int64 `json:"usage"`
	Total int64 `json:"total"`
}

type rawNetwork struct {
	Addresses []rawAddress `json:"addresses"`
	Hwaddr    string       `json:"hwaddr"`
	State     string       `json:"state"`
	Type      string       `json:"type"`
}

type rawAddress struct {
	Family  string `json:"family"`
	Address string `json:"address"`
	Netmask string `json:"netmask"`
	Scope   string `json:"scope"`
}

type rawOSInfo struct {
	OS            string `json:"os"`
	OSVersion     string `json:"os_version"`
	KernelVersion string `json:"kernel_version"`
	Hostname      string `json:"hostname"`
}

// ListInstances queries GET /1.0/instances?recursion=2 to get all instances
// with full state, resource counters, and network addresses in a single round-trip.
func (c *Client) ListInstances(ctx context.Context) ([]Instance, error) {
	url := fmt.Sprintf("%s/1.0/instances?recursion=2", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("incus request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("incus api returned status %d: %s", resp.StatusCode, string(body))
	}

	var res apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode incus response: %w", err)
	}

	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	instances := make([]Instance, 0, len(res.Metadata))
	for _, raw := range res.Metadata {
		inst := Instance{
			Name:       raw.Name,
			Type:       raw.Type,
			Manager:    "incus",
			Status:     raw.Status,
			StatusCode: raw.StatusCode,
			IsVM:       raw.Type == "virtual-machine",
			IPv4:       []string{},
			Config:     make(map[string]string),
		}

		// Config metadata
		if raw.Config != nil {
			inst.AutoStart = raw.Config["boot.autostart"] == "true"
			inst.Arch = raw.Config["image.architecture"]
			inst.CPUCores = parseCPUCores(raw.Config["limits.cpu"])

			for k, v := range raw.Config {
				if strings.HasPrefix(k, "limits.") || strings.HasPrefix(k, "boot.") ||
					strings.HasPrefix(k, "image.") || strings.HasPrefix(k, "security.") {
					inst.Config[k] = v
				}
			}
		}

		// Devices: extract disk pool and size, and bridge
		if raw.Devices != nil {
			if rootDev, ok := raw.Devices["root"]; ok {
				if pool, ok := rootDev["pool"].(string); ok {
					inst.DiskPool = pool
				}
				if sizeStr, ok := rootDev["size"].(string); ok {
					inst.DiskTotal = parseBytes(sizeStr)
				}
			}
			for _, dev := range raw.Devices {
				if devType, ok := dev["type"].(string); ok && devType == "nic" {
					if parent, ok := dev["parent"].(string); ok && parent != "" {
						inst.Bridge = parent
					}
					if hw, ok := dev["hwaddr"].(string); ok && hw != "" && inst.MAC == "" {
						inst.MAC = hw
					}
				}
			}
		}

		// State information
		if raw.State != nil {
			inst.StartedAt = raw.State.StartedAt

			// OS and Kernel info from guest agent
			if raw.State.OSInfo.OS != "" {
				inst.OS = strings.TrimSpace(fmt.Sprintf("%s %s", raw.State.OSInfo.OS, raw.State.OSInfo.OSVersion))
				inst.Kernel = raw.State.OSInfo.KernelVersion
			} else if raw.Config != nil {
				if desc := raw.Config["image.description"]; desc != "" {
					inst.OS = desc
				} else if osName := raw.Config["image.os"]; osName != "" {
					inst.OS = strings.TrimSpace(fmt.Sprintf("%s %s", osName, raw.Config["image.release"]))
				}
			}

			// Memory
			inst.MemUsed = raw.State.Memory.Usage
			if raw.State.Memory.Total > 0 {
				inst.MemTotal = raw.State.Memory.Total
			} else if raw.Config != nil && raw.Config["limits.memory"] != "" {
				inst.MemTotal = parseBytes(raw.Config["limits.memory"])
			}

			// Disk root usage
			if rootDisk, ok := raw.State.Disk["root"]; ok {
				inst.DiskUsed = rootDisk.Usage
				if rootDisk.Total > 0 && inst.DiskTotal == 0 {
					inst.DiskTotal = rootDisk.Total
				}
			}

			// Networking: collect global IPv4s and MAC
			for ifaceName, iface := range raw.State.Network {
				// Ignore virtual interface artifacts
				if ifaceName == "lo" || strings.HasPrefix(ifaceName, "docker") ||
					strings.HasPrefix(ifaceName, "cni") || strings.HasPrefix(ifaceName, "flannel") ||
					strings.HasPrefix(ifaceName, "veth") || strings.HasPrefix(ifaceName, "br-") {
					continue
				}
				if inst.MAC == "" && iface.Hwaddr != "" {
					inst.MAC = iface.Hwaddr
				}
				for _, addr := range iface.Addresses {
					if addr.Family == "inet" && addr.Scope == "global" && addr.Address != "" {
						inst.IPv4 = append(inst.IPv4, addr.Address)
					}
				}
			}

			// CPU % calculation across samples
			if raw.State.CPU.Usage > 0 {
				prev, hasPrev := c.prevCPU[raw.Name]
				if hasPrev {
					deltaNs := raw.State.CPU.Usage - prev.usageNs
					durationNs := now.Sub(prev.timestamp).Nanoseconds()
					if durationNs > 0 && deltaNs >= 0 {
						pct := (float64(deltaNs) / float64(durationNs)) * 100.0
						inst.CPUPercent = math.Round(pct*10) / 10
					}
				}
				c.prevCPU[raw.Name] = cpuSample{
					usageNs:   raw.State.CPU.Usage,
					timestamp: now,
				}
			}
		}

		instances = append(instances, inst)
	}

	vm.Sort(instances)

	// Forget CPU samples of instances that no longer exist, or the map
	// grows forever as instances are created and deleted.
	seen := make(map[string]struct{}, len(res.Metadata))
	for _, raw := range res.Metadata {
		seen[raw.Name] = struct{}{}
	}
	for name := range c.prevCPU {
		if _, ok := seen[name]; !ok {
			delete(c.prevCPU, name)
		}
	}

	return instances, nil
}

func parseCPUCores(val string) int {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	if n, err := strconv.Atoi(val); err == nil {
		return n
	}
	if strings.Contains(val, "-") {
		parts := strings.SplitN(val, "-", 2)
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil && end >= start {
				return end - start + 1
			}
		}
	}
	if strings.Contains(val, ",") {
		return len(strings.Split(val, ","))
	}
	return 0
}

func parseBytes(val string) int64 {
	val = strings.TrimSpace(strings.ToUpper(val))
	if val == "" {
		return 0
	}
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(val, "TIB"):
		multiplier = 1024 * 1024 * 1024 * 1024
		val = strings.TrimSuffix(val, "TIB")
	case strings.HasSuffix(val, "TB"):
		multiplier = 1000 * 1000 * 1000 * 1000
		val = strings.TrimSuffix(val, "TB")
	case strings.HasSuffix(val, "GIB"):
		multiplier = 1024 * 1024 * 1024
		val = strings.TrimSuffix(val, "GIB")
	case strings.HasSuffix(val, "GB"):
		multiplier = 1000 * 1000 * 1000
		val = strings.TrimSuffix(val, "GB")
	case strings.HasSuffix(val, "MIB"):
		multiplier = 1024 * 1024
		val = strings.TrimSuffix(val, "MIB")
	case strings.HasSuffix(val, "MB"):
		multiplier = 1000 * 1000
		val = strings.TrimSuffix(val, "MB")
	case strings.HasSuffix(val, "KIB"):
		multiplier = 1024
		val = strings.TrimSuffix(val, "KIB")
	case strings.HasSuffix(val, "KB"):
		multiplier = 1000
		val = strings.TrimSuffix(val, "KB")
	case strings.HasSuffix(val, "B"):
		val = strings.TrimSuffix(val, "B")
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if err != nil {
		return 0
	}
	return int64(n * float64(multiplier))
}
