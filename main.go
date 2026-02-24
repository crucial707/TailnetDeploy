// TailnetDeploy: one-shot Tailscale install, tailnet join, and RDP enable.
package main

import (
	"encoding/json"
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
	rdpRegistryPath     = `HKLM\System\CurrentControlSet\Control\Terminal Server`
	rdpRegistryValue    = "fDenyTSConnections"
)

func main() {
	// Auth
	authKey := flag.String("authkey", "", "Tailscale auth key (or set TS_AUTH_KEY or -authkey-file).")
	authKeyFile := flag.String("authkey-file", "", "Path to file containing auth key (first line used).")

	// Deploy flow
	skipInstall := flag.Bool("skip-install", false, "Skip Tailscale download/install; only run up + RDP.")
	skipRDP := flag.Bool("skip-rdp", false, "Skip enabling Remote Desktop.")
	version := flag.String("version", tailscaleVersion, "Tailscale MSI version to download (e.g. 1.94.2).")

	// Tailscale options (passed to tailscale up)
	exitNode := flag.Bool("exit-node", false, "Advertise this machine as an exit node.")
	advertiseRoutes := flag.String("advertise-routes", "", "Comma-separated routes to advertise (e.g. 192.168.1.0/24).")
	hostname := flag.String("hostname", "", "Hostname for this device (MagicDNS).")
	unattended := flag.Bool("unattended", true, "Run Tailscale in unattended mode (keeps running after logout).")

	// Output and verification
	healthCheck := flag.Bool("health-check", false, "Run health check after deploy; exit non-zero if not ready.")
	jsonOutput := flag.Bool("json", false, "Output machine report as JSON (IP, hostname, status).")
	dryRun := flag.Bool("dry-run", false, "Print planned steps without executing.")

	// Teardown
	teardown := flag.Bool("teardown", false, "Disable RDP and disconnect Tailscale (reverse of deploy).")
	teardownUninstall := flag.Bool("teardown-uninstall", false, "With -teardown, also uninstall Tailscale (requires -version if not default).")

	flag.Parse()

	if runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "This tool is for Windows only (Tailscale MSI + RDP).")
		os.Exit(1)
	}

	if *teardown {
		runTeardown(*teardownUninstall, *version, *dryRun)
		return
	}

	key := resolveAuthKey(*authKey, *authKeyFile)
	if key == "" && !*skipInstall {
		fmt.Fprintln(os.Stderr, "Provide -authkey=KEY, -authkey-file=path, or set TS_AUTH_KEY to join your tailnet.")
		flag.Usage()
		os.Exit(1)
	}

	if *dryRun {
		runDryRun(*version, key, *skipInstall, *skipRDP, *exitNode, *advertiseRoutes, *hostname, *unattended)
		return
	}

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
		time.Sleep(5 * time.Second)
	}

	if key != "" {
		fmt.Println("Connecting to tailnet...")
		if err := tailscaleUp(key, *hostname, *unattended, *exitNode, *advertiseRoutes); err != nil {
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

	if *healthCheck {
		if err := runHealthCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Health check passed.")
	}

	if *jsonOutput {
		report := buildReport()
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		return
	}

	fmt.Println("Done. You can RDP to this machine via its Tailscale IP.")
	if key != "" {
		printTailscaleIP()
	}
}

func resolveAuthKey(flagKey, keyFile string) string {
	if flagKey != "" {
		return strings.TrimSpace(flagKey)
	}
	if keyFile != "" {
		b, err := os.ReadFile(keyFile)
		if err != nil {
			return ""
		}
		line := strings.Split(string(b), "\n")[0]
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(os.Getenv("TS_AUTH_KEY"))
}

func runDryRun(version, key string, skipInstall, skipRDP, exitNode bool, advertiseRoutes, hostname string, unattended bool) {
	step := 0
	fmt.Println("[dry-run] Would perform the following:")
	if !skipInstall {
		arch := "amd64"
		if runtime.GOARCH == "arm64" {
			arch = "arm64"
		}
		url := fmt.Sprintf("%s/tailscale-setup-%s-%s.msi", tailscaleStableBase, version, arch)
		step++
		fmt.Printf("  %d. Download: %s\n", step, url)
		step++
		fmt.Printf("  %d. Install: msiexec /i <msi> /qn /norestart\n", step)
	}
	if key != "" {
		args := []string{"tailscale", "up", "--authkey=***"}
		if hostname != "" {
			args = append(args, "--hostname="+hostname)
		}
		if unattended {
			args = append(args, "--unattended")
		}
		if exitNode {
			args = append(args, "--advertise-exit-node")
		}
		if advertiseRoutes != "" {
			args = append(args, "--advertise-routes="+advertiseRoutes)
		}
		step++
		fmt.Printf("  %d. Connect: %s\n", step, strings.Join(args, " "))
	}
	if !skipRDP {
		step++
		fmt.Printf("  %d. Enable RDP: reg add %s /v %s /t REG_DWORD /d 0 /f\n", step, rdpRegistryPath, rdpRegistryValue)
		step++
		fmt.Printf("  %d. Firewall: Enable-NetFirewallRule -DisplayGroup \"Remote Desktop\"\n", step)
	}
	fmt.Println("[dry-run] No changes made.")
}

func runTeardown(uninstall bool, version string, dryRun bool) {
	if dryRun {
		fmt.Println("[dry-run] Teardown would: 1) Disable RDP 2) tailscale down 3) Uninstall Tailscale (if -teardown-uninstall)")
		return
	}
	fmt.Println("Disabling Remote Desktop...")
	if err := disableRDP(); err != nil {
		fmt.Fprintf(os.Stderr, "Disable RDP failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Disconnecting Tailscale...")
	_ = tailscaleDown()
	if uninstall {
		fmt.Println("Uninstalling Tailscale...")
		if err := uninstallTailscale(version); err != nil {
			fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println("Teardown complete.")
}

func tailscaleUp(authKey, hostname string, unattended, exitNode bool, advertiseRoutes string) error {
	exe := tailscaleExe
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("tailscale not found at %s: %w", exe, err)
	}
	args := []string{"up", "--authkey=" + authKey}
	if hostname != "" {
		args = append(args, "--hostname="+hostname)
	}
	if unattended {
		args = append(args, "--unattended")
	}
	if exitNode {
		args = append(args, "--advertise-exit-node")
	}
	if advertiseRoutes != "" {
		args = append(args, "--advertise-routes="+advertiseRoutes)
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func tailscaleDown() error {
	if _, err := os.Stat(tailscaleExe); err != nil {
		return nil
	}
	cmd := exec.Command(tailscaleExe, "down", "--logout")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

// statusJSON is a minimal view of tailscale status --json for Self and connectivity.
type statusJSON struct {
	Self *struct {
		HostName     string   `json:"HostName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`
	BackendState string `json:"BackendState"`
}

func buildReport() map[string]interface{} {
	report := map[string]interface{}{
		"tailscale_installed": false,
		"connected":           false,
		"ipv4":                "",
		"hostname":            "",
		"rdp_enabled":         rdpEnabled(),
	}
	if _, err := os.Stat(tailscaleExe); err != nil {
		return report
	}
	report["tailscale_installed"] = true
	cmd := exec.Command(tailscaleExe, "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return report
	}
	var st statusJSON
	if err := json.Unmarshal(out, &st); err != nil {
		return report
	}
	report["connected"] = st.BackendState == "Running"
	if st.Self != nil {
		for _, ip := range st.Self.TailscaleIPs {
			if !strings.Contains(ip, ":") {
				report["ipv4"] = ip
				break
			}
		}
		report["hostname"] = st.Self.HostName
	}
	if report["ipv4"] == "" && report["connected"] == true {
		if ipOut, err := exec.Command(tailscaleExe, "ip", "-4").Output(); err == nil {
			report["ipv4"] = strings.TrimSpace(string(ipOut))
		}
	}
	return report
}

func rdpEnabled() bool {
	cmd := exec.Command("reg", "query", rdpRegistryPath, "/v", rdpRegistryValue)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "0x0")
}

func runHealthCheck() error {
	if _, err := os.Stat(tailscaleExe); err != nil {
		return fmt.Errorf("tailscale not installed: %w", err)
	}
	cmd := exec.Command(tailscaleExe, "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("tailscale status: %w", err)
	}
	var st statusJSON
	if err := json.Unmarshal(out, &st); err != nil {
		return fmt.Errorf("parse status: %w", err)
	}
	if st.BackendState != "Running" {
		return fmt.Errorf("tailscale not connected (BackendState=%s)", st.BackendState)
	}
	// Optional: check RDP port 3389 is listening (TCP). On Windows we could netstat or Test-NetConnection.
	// Keep it simple: only check Tailscale is running.
	return nil
}

func downloadMSI(url string) (string, error) {
	dir := os.TempDir()
	base := filepath.Base(url)
	if i := strings.Index(base, "?"); i > 0 {
		base = base[:i]
	}
	path := filepath.Join(dir, base)
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

func enableRDP() error {
	cmd := exec.Command("reg", "add", rdpRegistryPath, "/v", rdpRegistryValue, "/t", "REG_DWORD", "/d", "0", "/f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	ps := `Enable-NetFirewallRule -DisplayGroup "Remote Desktop"`
	cmd2 := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	if err := cmd2.Run(); err != nil {
		return fmt.Errorf("firewall: %w", err)
	}
	return nil
}

func disableRDP() error {
	cmd := exec.Command("reg", "add", rdpRegistryPath, "/v", rdpRegistryValue, "/t", "REG_DWORD", "/d", "1", "/f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	ps := `Disable-NetFirewallRule -DisplayGroup "Remote Desktop"`
	cmd2 := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	if err := cmd2.Run(); err != nil {
		return fmt.Errorf("firewall: %w", err)
	}
	return nil
}

func uninstallTailscale(version string) error {
	arch := "amd64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	url := fmt.Sprintf("%s/tailscale-setup-%s-%s.msi", tailscaleStableBase, version, arch)
	path, err := downloadMSI(url)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	cmd := exec.Command("msiexec.exe", "/x", path, "/qn", "/norestart")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
