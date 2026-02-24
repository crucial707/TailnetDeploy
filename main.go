// TailnetDeploy: one-shot Tailscale install, tailnet join, and RDP enable.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	tailscaleStableBase = "https://pkgs.tailscale.com/stable"
	tailscaleVersion    = "1.94.2"
	tailscaleExe        = `C:\Program Files\Tailscale\tailscale.exe`
)

func main() {
	authKey := flag.String("authkey", "", "Tailscale auth key (or set TS_AUTH_KEY). Required to join tailnet.")
	skipInstall := flag.Bool("skip-install", false, "Skip Tailscale download/install; only run up + RDP.")
	skipRDP := flag.Bool("skip-rdp", false, "Skip enabling Remote Desktop.")
	version := flag.String("version", tailscaleVersion, "Tailscale MSI version to download (e.g. 1.94.2).")
	flag.Parse()

	if runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "This tool is for Windows only (Tailscale MSI + RDP).")
		os.Exit(1)
	}

	key := *authKey
	if key == "" {
		key = os.Getenv("TS_AUTH_KEY")
	}
	if key == "" && !*skipInstall {
		fmt.Fprintln(os.Stderr, "Provide -authkey=KEY or set TS_AUTH_KEY to join your tailnet.")
		flag.Usage()
		os.Exit(1)
	}
	// If only enabling RDP (skip-install, no key), that's allowed.

	var msiPath string
	if !*skipInstall {
		arch := "amd64"
		if runtime.GOARCH == "arm64" {
			arch = "arm64"
		}
		url := fmt.Sprintf("%s/tailscale-setup-%s-%s.msi", tailscaleStableBase, *version, arch)
		fmt.Println("Downloading Tailscale...")
		var err error
		msiPath, err = downloadMSI(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(msiPath)
		fmt.Println("Installing Tailscale (silent)...")
		if err := installMSI(msiPath); err != nil {
			fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
			os.Exit(1)
		}
		// Give the Tailscale service time to start.
		time.Sleep(5 * time.Second)
	}

	if key != "" {
		fmt.Println("Connecting to tailnet...")
		if err := tailscaleUp(key); err != nil {
			fmt.Fprintf(os.Stderr, "tailscale up failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Connected to tailnet.")
	}

	if !*skipRDP {
		fmt.Println("Enabling Remote Desktop...")
		if err := enableRDP(); err != nil {
			fmt.Fprintf(os.Stderr, "Enable RDP failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Remote Desktop is enabled.")
	}

	fmt.Println("Done. You can RDP to this machine via its Tailscale IP.")
	if key != "" {
		printTailscaleIP()
	}
}

func printTailscaleIP() {
	cmd := exec.Command(tailscaleExe, "ip", "-4")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	ip := strings.TrimSpace(string(out))
	if ip != "" {
		fmt.Printf("Tailscale IPv4: %s (use this for RDP)\n", ip)
	}
}

func downloadMSI(url string) (string, error) {
	dir := os.TempDir()
	base := filepath.Base(url)
	if i := strings.Index(base, "?"); i > 0 {
		base = base[:i]
	}
	path := filepath.Join(dir, base)

	// Use PowerShell to download so we don't need net/http with TLS.
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Invoke-WebRequest -Uri %q -OutFile %q -UseBasicParsing", url, path))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func installMSI(path string) error {
	cmd := exec.Command("msiexec.exe", "/i", path, "/qn", "/norestart")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func tailscaleUp(authKey string) error {
	exe := tailscaleExe
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("tailscale not found at %s: %w", exe, err)
	}
	cmd := exec.Command(exe, "up", "--authkey="+authKey)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func enableRDP() error {
	// 1) Enable RDP in registry: fDenyTSConnections = 0
	cmd := exec.Command("reg", "add",
		`HKLM\System\CurrentControlSet\Control\Terminal Server`,
		"/v", "fDenyTSConnections", "/t", "REG_DWORD", "/d", "0", "/f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	// 2) Allow RDP in Windows Firewall
	ps := `Enable-NetFirewallRule -DisplayGroup "Remote Desktop"`
	cmd2 := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	if err := cmd2.Run(); err != nil {
		return fmt.Errorf("firewall: %w", err)
	}
	return nil
}
