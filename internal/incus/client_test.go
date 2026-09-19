package incus

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func generateTestCertAndKey(t *testing.T) (certPEM, keyPEM string) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-client",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	certBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}
	keyBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return string(certBlock), string(keyBlock)
}

func TestParseCPUCores(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"4", 4},
		{"0-3", 4},
		{"2-5", 4},
		{"0,1,2", 3},
		{"invalid", 0},
	}

	for _, tt := range tests {
		got := parseCPUCores(tt.input)
		if got != tt.want {
			t.Errorf("parseCPUCores(%q) = %d; want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseBytes(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"", 0},
		{"20GiB", 20 * 1024 * 1024 * 1024},
		{"6144MiB", 6144 * 1024 * 1024},
		{"2GB", 2 * 1000 * 1000 * 1000},
		{"512MB", 512 * 1000 * 1000},
		{"1024B", 1024},
		{"invalid", 0},
	}

	for _, tt := range tests {
		got := parseBytes(tt.input)
		if got != tt.want {
			t.Errorf("parseBytes(%q) = %d; want %d", tt.input, got, tt.want)
		}
	}
}

func TestParsePEMOrFile(t *testing.T) {
	certPEM, _ := generateTestCertAndKey(t)

	// 1. Raw PEM
	data, err := parsePEMOrFile(certPEM)
	if err != nil || strings.TrimSpace(string(data)) != strings.TrimSpace(certPEM) {
		t.Fatalf("parsePEMOrFile with raw PEM failed: %v", err)
	}

	// 2. Base64-encoded PEM
	b64 := base64.StdEncoding.EncodeToString([]byte(certPEM))
	data, err = parsePEMOrFile(b64)
	if err != nil || strings.TrimSpace(string(data)) != strings.TrimSpace(certPEM) {
		t.Fatalf("parsePEMOrFile with base64 failed: %v", err)
	}

	// 3. File path
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "client.crt")
	if err := os.WriteFile(certPath, []byte(certPEM), 0600); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}
	data, err = parsePEMOrFile(certPath)
	if err != nil || strings.TrimSpace(string(data)) != strings.TrimSpace(certPEM) {
		t.Fatalf("parsePEMOrFile with file path failed: %v", err)
	}
}

func TestListInstances(t *testing.T) {
	certPEM, keyPEM := generateTestCertAndKey(t)

	mockResponse := apiResponse{
		Type:       "sync",
		Status:     "Success",
		StatusCode: 200,
		Metadata: []rawInstance{
			{
				Name:       "test-vm",
				Type:       "virtual-machine",
				Status:     "Running",
				StatusCode: 103,
				Config: map[string]string{
					"limits.cpu":         "3",
					"limits.memory":      "6144MiB",
					"boot.autostart":     "true",
					"image.architecture": "x86_64",
					"image.os":           "Ubuntu",
					"image.release":      "noble",
				},
				Devices: map[string]map[string]interface{}{
					"root": {
						"type": "disk",
						"pool": "fast",
						"size": "20GiB",
					},
					"eth0": {
						"type":   "nic",
						"parent": "br0",
						"hwaddr": "02:00:00:00:00:01",
					},
				},
				State: &rawState{
					Status:     "Running",
					StatusCode: 103,
					StartedAt:  "2026-09-14T18:04:52Z",
					CPU:        rawCPU{Usage: 1000000000}, // 1 second of CPU
					Memory:     rawMemory{Usage: 2000000000, Total: 6442450944},
					Disk: map[string]rawDisk{
						"root": {Usage: 15000000000, Total: 21474836480},
					},
					Network: map[string]rawNetwork{
						"eth0": {
							Hwaddr: "02:00:00:00:00:01",
							Addresses: []rawAddress{
								{Family: "inet", Address: "192.168.1.140", Scope: "global"},
							},
						},
					},
					OSInfo: rawOSInfo{
						OS:            "Ubuntu",
						OSVersion:     "24.04",
						KernelVersion: "6.8.0-139-generic",
					},
				},
			},
			{
				Name:       "test-container",
				Type:       "container",
				Status:     "Running",
				StatusCode: 103,
				Config: map[string]string{
					"limits.cpu": "2",
				},
				State: &rawState{
					Status: "Running",
					CPU:    rawCPU{Usage: 500000000},
					Memory: rawMemory{Usage: 500000000, Total: 2147483648},
					Disk: map[string]rawDisk{
						"root": {Usage: 1000000000},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1.0/instances" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("recursion") != "2" {
			t.Errorf("expected recursion=2, got: %s", r.URL.Query().Get("recursion"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, certPEM, keyPEM, true)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	instances, err := client.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}

	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	// VM must be first (sorting rule: VMs first, then containers)
	vm := instances[0]
	if !vm.IsVM || vm.Name != "test-vm" {
		t.Errorf("expected test-vm to be first, got %+v", vm)
	}
	if vm.CPUCores != 3 {
		t.Errorf("expected 3 CPU cores, got %d", vm.CPUCores)
	}
	if vm.OS != "Ubuntu 24.04" {
		t.Errorf("expected OS 'Ubuntu 24.04', got %q", vm.OS)
	}
	if vm.Kernel != "6.8.0-139-generic" {
		t.Errorf("expected Kernel '6.8.0-139-generic', got %q", vm.Kernel)
	}
	if vm.DiskPool != "fast" {
		t.Errorf("expected DiskPool 'fast', got %q", vm.DiskPool)
	}
	if vm.Bridge != "br0" {
		t.Errorf("expected Bridge 'br0', got %q", vm.Bridge)
	}
	if vm.MAC != "02:00:00:00:00:01" {
		t.Errorf("expected MAC '02:00:00:00:00:01', got %q", vm.MAC)
	}
	if len(vm.IPv4) != 1 || vm.IPv4[0] != "192.168.1.140" {
		t.Errorf("expected IPv4 ['192.168.1.140'], got %v", vm.IPv4)
	}
	if !vm.AutoStart {
		t.Errorf("expected AutoStart true, got false")
	}

	ct := instances[1]
	if ct.IsVM || ct.Name != "test-container" {
		t.Errorf("expected test-container to be second, got %+v", ct)
	}
}

func TestLiveIncusConnection(t *testing.T) {
	rawURL := os.Getenv("INCUS_URL")
	certData := os.Getenv("INCUS_CLIENT_CERT")
	keyData := os.Getenv("INCUS_CLIENT_KEY")
	if rawURL == "" || certData == "" || keyData == "" {
		t.Skip("skipping live Incus test: INCUS_URL, INCUS_CLIENT_CERT, and INCUS_CLIENT_KEY must be set")
	}

	client, err := NewClient(rawURL, certData, keyData, true)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	instances, err := client.ListInstances(ctx)
	if err != nil {
		t.Fatalf("ListInstances failed against live Incus: %v", err)
	}

	if len(instances) == 0 {
		t.Fatalf("expected at least 1 instance on live host, got 0")
	}

	for _, inst := range instances {
		t.Logf("Instance %q (type: %s, is_vm: %v, status: %s, os: %q, ip: %v, cores: %d, mem: %d/%d, disk: %d/%d)",
			inst.Name, inst.Type, inst.IsVM, inst.Status, inst.OS, inst.IPv4, inst.CPUCores, inst.MemUsed, inst.MemTotal, inst.DiskUsed, inst.DiskTotal)
	}
}

func TestPrevCPUPrunedForDeletedInstances(t *testing.T) {
	c := &Client{prevCPU: map[string]cpuSample{"gone": {usageNs: 1}, "kept": {usageNs: 1}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"metadata":[{"name":"kept","type":"container","status":"Running","state":{"cpu":{"usage":5}}}]}`))
	}))
	defer srv.Close()
	c.baseURL = srv.URL
	c.http = srv.Client()
	if _, err := c.ListInstances(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.prevCPU["gone"]; ok {
		t.Fatal("sample for a deleted instance should be pruned")
	}
	if _, ok := c.prevCPU["kept"]; !ok {
		t.Fatal("sample for a live instance should be kept")
	}
}
