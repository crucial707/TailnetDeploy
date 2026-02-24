# TailnetDeploy

One-shot Windows CLI that downloads Tailscale, connects to your tailnet, and enables Remote Desktop (RDP). Run as **Administrator**.

## Quick start

1. Create an [auth key](https://login.tailscale.com/admin/settings/keys) in the Tailscale admin console.
2. Open PowerShell or CMD **as Administrator**.
3. Build and run:

```powershell
go build -o tailnetdeploy.exe .
.\tailnetdeploy.exe -authkey=tskey-auth-xxxxx
```

Or use env (keeps key out of process list):

```powershell
$env:TS_AUTH_KEY = "tskey-auth-xxxxx"
.\tailnetdeploy.exe
```

Use **-json** to get a machine report (IP, hostname, status) for scripting.

---

## One-liner installer

From the repo directory (after cloning), run the helper script as Administrator:

```powershell
# Uses local tailnetdeploy.exe if present; otherwise prompts to build
.\install.ps1
```

With auth key:

```powershell
$env:TS_AUTH_KEY = "tskey-auth-xxxxx"; .\install.ps1
```

To host your own one-liner: publish `tailnetdeploy.exe` as a release asset, set the URL in `install.ps1`, then:

```powershell
irm https://your-domain.com/install.ps1 | iex
```

---

## All options

### Auth

| Flag | Description |
|------|-------------|
| `-authkey=KEY` | Tailscale auth key. |
| `-authkey-file=path` | Read auth key from file (first line). |
| `TS_AUTH_KEY` | Env var for auth key (same as above). |

### Deploy flow

| Flag | Description |
|------|-------------|
| `-skip-install` | Do not download/install Tailscale; only run `tailscale up` and/or RDP. |
| `-skip-rdp` | Do not enable Remote Desktop. |
| `-version=V` | Tailscale MSI version to download (default: `1.94.2`). |

### Tailscale (passed to `tailscale up`)

| Flag | Description |
|------|-------------|
| `-exit-node` | Advertise this machine as an exit node. |
| `-advertise-routes=routes` | Comma-separated subnet routes (e.g. `192.168.1.0/24,10.0.0.0/8`). |
| `-hostname=name` | Hostname for this device (MagicDNS). |
| `-unattended` | Run Tailscale in unattended mode (default: true; keeps running after user logout). |

### Output and verification

| Flag | Description |
|------|-------------|
| `-health-check` | After deploy, verify Tailscale is connected; exit non-zero if not. |
| `-json` | Output machine report as JSON (ipv4, hostname, connected, rdp_enabled). |
| `-dry-run` | Print planned steps only; no download, install, or registry changes. |

### Teardown (reverse of deploy)

| Flag | Description |
|------|-------------|
| `-teardown` | Disable RDP and run `tailscale down --logout`. |
| `-teardown-uninstall` | With `-teardown`, also uninstall Tailscale (downloads MSI and runs msiexec /x). |

---

## Examples

**Full deploy with exit node and custom hostname:**

```powershell
.\tailnetdeploy.exe -authkey=tskey-auth-xxxxx -exit-node -hostname=desktop-office
```

**Subnet router (advertise local LAN):**

```powershell
.\tailnetdeploy.exe -authkey=tskey-auth-xxxxx -advertise-routes=192.168.1.0/24
```

**Auth key from file (e.g. in automation):**

```powershell
.\tailnetdeploy.exe -authkey-file=C:\secrets\ts-key.txt
```

**Deploy and verify; output JSON for scripts:**

```powershell
.\tailnetdeploy.exe -authkey=tskey-auth-xxxxx -health-check -json
```

**See what would happen without making changes:**

```powershell
.\tailnetdeploy.exe -authkey=tskey-auth-xxxxx -dry-run
```

**Teardown: disable RDP and disconnect Tailscale:**

```powershell
.\tailnetdeploy.exe -teardown
```

**Teardown and uninstall Tailscale:**

```powershell
.\tailnetdeploy.exe -teardown -teardown-uninstall
```

**Only enable RDP** (Tailscale already installed):

```powershell
.\tailnetdeploy.exe -skip-install
```

---

## Requirements

- **Windows 10+** or **Windows Server 2016+**
- **Administrator** rights (install, registry, firewall)
- Network access to `pkgs.tailscale.com` and Tailscale coordination servers

## How it works

- **Download:** Fetches `tailscale-setup-<version>-amd64.msi` (or arm64) from `https://pkgs.tailscale.com/stable/`.
- **Install:** `msiexec /i ... /qn /norestart`.
- **Connect:** `tailscale up --authkey=...` with optional `--hostname`, `--unattended`, `--advertise-exit-node`, `--advertise-routes`.
- **RDP:** Sets `HKLM\...\Terminal Server\fDenyTSConnections = 0` and enables the “Remote Desktop” firewall group.
- **Teardown:** Sets RDP to disabled, runs `tailscale down --logout`; with `-teardown-uninstall`, downloads MSI and runs `msiexec /x`.

## License

MIT
