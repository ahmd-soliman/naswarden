package truenas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestDetectOS(t *testing.T) {
	tests := []struct {
		name        string
		vmName      string
		cdromPaths  []string
		description string
		want        string
	}{
		{
			name:        "Windows 10 22H2 ISO with virtio drivers",
			vmName:      "win",
			cdromPaths:  []string{"/mnt/tank/main/misc/virtio-win-0.1.266.iso", "/mnt/tank/main/misc/Win10_22H2_EnglishInternational_x64v1.iso"},
			description: "",
			want:        "Windows 10 (22H2)",
		},
		{
			name:        "Windows 11 23H2 ISO",
			vmName:      "win11-workstation",
			cdromPaths:  []string{"/mnt/tank/Win11_23H2_English.iso"},
			description: "",
			want:        "Windows 11 (23H2)",
		},
		{
			name:        "Ubuntu 24.04 ISO",
			vmName:      "ubuntu-vm",
			cdromPaths:  []string{"/mnt/tank/ubuntu-24.04-live-server-amd64.iso"},
			description: "",
			want:        "Ubuntu 24.04",
		},
		{
			name:        "Debian 12 ISO",
			vmName:      "deb",
			cdromPaths:  []string{"/mnt/tank/debian-12.5.0-amd64-netinst.iso"},
			description: "",
			want:        "Debian 12",
		},
		{
			name:        "Fallback to description",
			vmName:      "custom-box",
			cdromPaths:  nil,
			description: "Custom Alpine Linux 3.20",
			want:        "Custom Alpine Linux 3.20",
		},
		{
			name:        "Fallback to VM name containing win",
			vmName:      "win-gaming",
			cdromPaths:  nil,
			description: "",
			want:        "Windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectOS(tt.vmName, tt.cdromPaths, tt.description)
			if got != tt.want {
				t.Errorf("detectOS(%q, %v, %q) = %q; want %q", tt.vmName, tt.cdromPaths, tt.description, got, tt.want)
			}
		})
	}
}

func TestFormatPCIDevice(t *testing.T) {
	tests := []struct {
		name   string
		choice pciChoice
		want   string
	}{
		{
			name: "NVIDIA GeForce GTX 1050 Ti with bracket model",
			choice: pciChoice{
				Description:    "0000:10:00.0 'VGA compatible controller': GP107 [GeForce GTX 1050 Ti] by 'NVIDIA Corporation'",
				ControllerType: "VGA compatible controller",
				Capability: struct {
					Product string `json:"product"`
					Vendor  string `json:"vendor"`
				}{
					Product: "GP107 [GeForce GTX 1050 Ti]",
					Vendor:  "NVIDIA Corporation",
				},
			},
			want: "NVIDIA GeForce GTX 1050 Ti",
		},
		{
			name: "NVIDIA High Definition Audio Controller",
			choice: pciChoice{
				Description:    "0000:10:00.1 'Audio device': GP107GL High Definition Audio Controller by 'NVIDIA Corporation'",
				ControllerType: "Audio device",
				Capability: struct {
					Product string `json:"product"`
					Vendor  string `json:"vendor"`
				}{
					Product: "GP107GL High Definition Audio Controller",
					Vendor:  "NVIDIA Corporation",
				},
			},
			want: "NVIDIA High Definition Audio Controller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPCIDevice(tt.choice)
			if got != tt.want {
				t.Errorf("formatPCIDevice() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestListVMsMock(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mockVMQueryJSON := `[
		{
			"id": 5,
			"name": "win",
			"description": "Windows Desktop",
			"vcpus": 1,
			"cores": 4,
			"threads": 2,
			"cpu_mode": "HOST-PASSTHROUGH",
			"memory": 32768,
			"autostart": false,
			"bootloader": "UEFI",
			"trusted_platform_module": true,
			"hyperv_enlightenments": true,
			"status": {
				"state": "STOPPED",
				"domain_state": "SHUTOFF",
				"pid": null
			},
			"devices": [
				{
					"id": 38,
					"attributes": {
						"dtype": "NIC",
						"type": "VIRTIO",
						"nic_attach": "br0",
						"mac": "02:00:00:00:00:03"
					}
				},
				{
					"id": 39,
					"attributes": {
						"dtype": "DISK",
						"path": "/dev/zvol/fast/vms/winvm-abc123",
						"type": "AHCI"
					}
				},
				{
					"id": 40,
					"attributes": {
						"dtype": "DISPLAY",
						"port": 5901,
						"web_port": 5902,
						"type": "SPICE",
						"resolution": "1024x768"
					}
				},
				{
					"id": 41,
					"attributes": {
						"dtype": "CDROM",
						"path": "/mnt/tank/main/misc/Win10_22H2_EnglishInternational_x64v1.iso"
					}
				},
				{
					"id": 168,
					"attributes": {
						"dtype": "PCI",
						"pptdev": "pci_0000_10_00_0"
					}
				}
			]
		}
	]`

	mockZvolQueryJSON := `[
		{
			"name": "fast/vms/winvm-abc123",
			"volsize": {"parsed": 1099511627776},
			"used": {"parsed": 130434760704}
		}
	]`

	mockPCIQueryJSON := `{
		"pci_0000_10_00_0": {
			"description": "0000:10:00.0 'VGA compatible controller': GP107 [GeForce GTX 1050 Ti] by 'NVIDIA Corporation'",
			"controller_type": "VGA compatible controller",
			"capability": {
				"product": "GP107 [GeForce GTX 1050 Ti]",
				"vendor": "NVIDIA Corporation"
			}
		}
	}`

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer wsConn.Close()

		for {
			var msg ddpMessage
			if err := wsConn.ReadJSON(&msg); err != nil {
				return
			}
			switch msg.Msg {
			case "connect":
				_ = wsConn.WriteJSON(ddpMessage{Msg: "connected"})
			case "method":
				switch msg.Method {
				case "auth.login_with_api_key":
					_ = wsConn.WriteJSON(ddpMessage{Msg: "result", ID: msg.ID, Result: json.RawMessage(`true`)})
				case "vm.query":
					_ = wsConn.WriteJSON(ddpMessage{Msg: "result", ID: msg.ID, Result: json.RawMessage(mockVMQueryJSON)})
				case "pool.dataset.query":
					_ = wsConn.WriteJSON(ddpMessage{Msg: "result", ID: msg.ID, Result: json.RawMessage(mockZvolQueryJSON)})
				case "vm.device.passthrough_device_choices":
					_ = wsConn.WriteJSON(ddpMessage{Msg: "result", ID: msg.ID, Result: json.RawMessage(mockPCIQueryJSON)})
				default:
					_ = wsConn.WriteJSON(ddpMessage{Msg: "result", ID: msg.ID, Result: json.RawMessage(`null`)})
				}
			}
		}
	}))
	defer s.Close()

	host := strings.TrimPrefix(s.URL, "http://")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Connect(ctx, host, "test-api-key", false, false)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	vms, err := ListVMs(ctx, client)
	if err != nil {
		t.Fatalf("ListVMs failed: %v", err)
	}

	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}

	win := vms[0]
	if win.Name != "win" {
		t.Errorf("expected name 'win', got %q", win.Name)
	}
	if win.Manager != "truenas" {
		t.Errorf("expected manager 'truenas', got %q", win.Manager)
	}
	if win.Status != "Stopped" || win.StatusCode != 102 {
		t.Errorf("expected status 'Stopped' (102), got %q (%d)", win.Status, win.StatusCode)
	}
	if win.CPUCores != 8 {
		t.Errorf("expected 8 vCPUs (1 socket * 4 cores * 2 threads), got %d", win.CPUCores)
	}
	if win.MemTotal != 32768*1024*1024 {
		t.Errorf("expected %d bytes RAM, got %d", 32768*1024*1024, win.MemTotal)
	}
	if win.OS != "Windows 10 (22H2)" {
		t.Errorf("expected OS 'Windows 10 (22H2)', got %q", win.OS)
	}
	if win.Bridge != "br0" || win.MAC != "02:00:00:00:00:03" {
		t.Errorf("expected bridge br0 / mac 02:00:00:00:00:03, got %s / %s", win.Bridge, win.MAC)
	}
	if win.DisplayPort != 5901 || win.WebPort != 5902 {
		t.Errorf("expected display ports 5901 / 5902, got %d / %d", win.DisplayPort, win.WebPort)
	}
	if win.DiskPool != "fast" || win.DiskTotal != 1099511627776 || win.DiskUsed != 130434760704 {
		t.Errorf("unexpected disk stats: pool=%s, total=%d, used=%d", win.DiskPool, win.DiskTotal, win.DiskUsed)
	}
	if len(win.Passthrough) != 1 || win.Passthrough[0] != "NVIDIA GeForce GTX 1050 Ti" {
		t.Errorf("expected passthrough ['NVIDIA GeForce GTX 1050 Ti'], got %v", win.Passthrough)
	}
}

func TestLiveTrueNASVMs(t *testing.T) {
	host := os.Getenv("TRUENAS_HOST")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	if host == "" || apiKey == "" {
		t.Skip("skipping live TrueNAS VM test: TRUENAS_HOST and TRUENAS_API_KEY must be set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	useTLS := os.Getenv("TRUENAS_TLS") == "true"
	insecureTLS := os.Getenv("TRUENAS_INSECURE_TLS") != "false"

	client, err := Connect(ctx, host, apiKey, useTLS, insecureTLS)
	if err != nil {
		t.Fatalf("failed to connect to TrueNAS: %v", err)
	}
	defer client.Close()

	vms, err := ListVMs(ctx, client)
	if err != nil {
		t.Fatalf("ListVMs failed: %v", err)
	}

	t.Logf("retrieved %d TrueNAS VMs", len(vms))
	for _, v := range vms {
		t.Logf("VM: name=%s manager=%s status=%s os=%s cores=%d ram=%d diskPool=%s diskUsed=%d diskTotal=%d pci=%v display=%d/%d mac=%s bridge=%s",
			v.Name, v.Manager, v.Status, v.OS, v.CPUCores, v.MemTotal, v.DiskPool, v.DiskUsed, v.DiskTotal, v.Passthrough, v.DisplayPort, v.WebPort, v.MAC, v.Bridge)
	}
}

