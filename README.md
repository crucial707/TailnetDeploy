# TailnetDeploy

One-shot Windows CLI that:

1. **Downloads** the latest Tailscale MSI from the stable channel  
2. **Installs** Tailscale silently  
3. **Connects** the machine to your tailnet using an auth key  
4. **Enables** Remote Desktop (RDP) and opens the firewall  

Run as **Administrator** (needed for install, registry, and firewall).

## Quick start

1. Create an [auth key](https://login.tailscale.com/admin/settings/keys) in the Tailscale admin console (one-off or reusable).
2. Open PowerShell or CMD **as Administrator** in this folder.
3. Run:

```powershell
# Build (once)
go build -o tailnetdeploy.exe .

# Full run: install Tailscale, join tailnet, enable RDP
.\tailnetdeploy.exe -authkey=tskey-auth-xxxxx
```

Or use an environment variable (keeps the key out of process lists):

```powershell
$env:TS_AUTH_KEY = "tskey-auth-xxxxx"
.\tailnetdeploy.exe
```

When it finishes, it prints the machine’s **Tailscale IPv4**; use that address for RDP from any device on your tailnet.

## Options

| Flag | Description |
|------|-------------|
| `-authkey=KEY` | Tailscale auth key (or set `TS_AUTH_KEY`). Required when installing so the machine can join your tailnet. |
| `-skip-install` | Do not download or install Tailscale (e.g. already installed). Only run `tailscale up` and/or RDP steps. |
| `-skip-rdp` | Do not enable Remote Desktop; only install and connect Tailscale. |
| `-version=V` | Tailscale MSI version to download (default: `1.94.2`). |

## Examples

- **Install + join + RDP (default):**  
  `tailnetdeploy.exe -authkey=tskey-auth-xxxxx`

- **Already have Tailscale, just join and enable RDP:**  
  `tailnetdeploy.exe -authkey=tskey-auth-xxxxx -skip-install`

- **Only enable RDP** (Tailscale already installed/connected):  
  `tailnetdeploy.exe -skip-install`  
  (Omit `-authkey`; only the RDP step runs.)

## Requirements

- **Windows 10+** or **Windows Server 2016+**
- **Administrator** rights (install, registry, firewall)
- Network access to `pkgs.tailscale.com` and Tailscale coordination servers

## How it works

- **Download:** Fetches `tailscale-setup-<version>-amd64.msi` (or `arm64` on ARM) from `https://pkgs.tailscale.com/stable/`.
- **Install:** Runs `msiexec /i ... /qn /norestart`.
- **Connect:** Runs `"C:\Program Files\Tailscale\tailscale.exe" up --authkey=...`.
- **RDP:** Sets `HKLM\...\Terminal Server\fDenyTSConnections = 0` and enables the “Remote Desktop” firewall group.

## License

MIT
