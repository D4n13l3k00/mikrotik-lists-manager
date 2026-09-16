package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertNativeToMikrotik(t *testing.T) {
	tempDir := t.TempDir()
	inFile := filepath.Join(tempDir, "source.list")
	outFile := filepath.Join(tempDir, "target.rsc")

	inputContent := `# Test list
1.1.1.1 ## Cloudflare
!8.8.8.8 ## Google DNS
192.168.0.0/16
`
	if err := os.WriteFile(inFile, []byte(inputContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := rootCmd
	cmd.SetArgs([]string{"convert", inFile, "-f", "mikrotik", "-l", "vpn-test", "-o", outFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "/ip firewall address-list") {
		t.Errorf("expected header /ip firewall address-list, got:\n%s", content)
	}
	if !strings.Contains(content, `add list=vpn-test address=1.1.1.1 comment="Cloudflare"`) {
		t.Errorf("expected 1.1.1.1 entry, got:\n%s", content)
	}
	if !strings.Contains(content, `add list=vpn-test address=8.8.8.8 comment="Google DNS" disabled=yes`) {
		t.Errorf("expected 8.8.8.8 disabled entry, got:\n%s", content)
	}
	if !strings.Contains(content, "add list=vpn-test address=192.168.0.0/16") {
		t.Errorf("expected 192.168.0.0/16 entry, got:\n%s", content)
	}
}

func TestConvertMikrotikToNative(t *testing.T) {
	tempDir := t.TempDir()
	inFile := filepath.Join(tempDir, "export.rsc")
	outFile := filepath.Join(tempDir, "out.list")

	inputContent := `/ip firewall address-list
add list=vpn-test address=1.1.1.1 comment="Cloudflare"
add list=vpn-test address=8.8.8.8 comment="Google DNS" disabled=yes
add list=vpn-test address=192.168.0.0/16
`
	if err := os.WriteFile(inFile, []byte(inputContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := rootCmd
	cmd.SetArgs([]string{"convert", inFile, "-f", "native", "-o", outFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "1.1.1.1  ## Cloudflare") {
		t.Errorf("expected 1.1.1.1 with comment, got:\n%s", content)
	}
	if !strings.Contains(content, "!8.8.8.8  ## Google DNS") {
		t.Errorf("expected disabled !8.8.8.8 with comment, got:\n%s", content)
	}
	if !strings.Contains(content, "192.168.0.0/16") {
		t.Errorf("expected 192.168.0.0/16, got:\n%s", content)
	}
}

func TestConvertAutoDetection(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Native -> Mikrotik
	nativeFile := filepath.Join(tempDir, "servers.list")
	if err := os.WriteFile(nativeFile, []byte("10.0.0.1 ## Internal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outRsc := filepath.Join(tempDir, "servers.rsc")

	cmd := rootCmd
	cmd.SetArgs([]string{"convert", nativeFile, "-o", outRsc})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert auto failed: %v", err)
	}
	data, err := os.ReadFile(outRsc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "add list=servers address=10.0.0.1") {
		t.Errorf("auto convert from native should use filename stem 'servers' and mikrotik format, got:\n%s", string(data))
	}

	// 2. Mikrotik -> Native
	outNative := filepath.Join(tempDir, "servers_back.list")
	cmd.SetArgs([]string{"convert", outRsc, "-o", outNative})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert auto from rsc failed: %v", err)
	}
	dataNative, err := os.ReadFile(outNative)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dataNative), "10.0.0.1  ## Internal") {
		t.Errorf("auto convert from mikrotik should produce native, got:\n%s", string(dataNative))
	}
}

func TestConvertWriteInPlace(t *testing.T) {
	tempDir := t.TempDir()
	f := filepath.Join(tempDir, "in_place.lst")
	if err := os.WriteFile(f, []byte("10.1.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := rootCmd
	cmd.SetArgs([]string{"convert", f, "-w", "-f", "mikrotik", "-l", "testlist"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert -w failed: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "add list=testlist address=10.1.1.1") {
		t.Errorf("expected file to be updated in-place, got:\n%s", string(data))
	}

	// Should error if both -w and -o are given
	cmd.SetArgs([]string{"convert", f, "-w", "-o", "some_out.rsc"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error when both -w and -o are specified")
	}
}
