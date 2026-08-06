# Installation

How to get `scrollshot` installed and ready to run. For commands, flags, and workflows once it's installed, see [`docs/usage.md`](usage.md).

---

## Pre-built binary

Download the latest release binary from the [Releases page](https://github.com/AkshatRauthan/ScrollShot/releases) and place it somewhere on your `PATH` — see [Adding to PATH](#adding-to-path-run-scrollshot-from-any-terminal) below.

## Package manager (Linux)

Each release also publishes a `.deb` and a `.rpm`, built for `amd64`. Download the one matching your distro from the [Releases page](https://github.com/AkshatRauthan/ScrollShot/releases) and install it:

```bash
# Debian, Ubuntu, Mint, Pop!_OS, etc.
sudo dpkg -i scrollshot-amd64.deb

# Fedora, openSUSE, RHEL, etc.
sudo rpm -i scrollshot-amd64.rpm
```

This installs the `scrollshot` binary to `/usr/bin`, so it's on your `PATH` immediately — no manual step needed.

> These aren't yet available through `apt`/`dnf` directly (that needs a hosted repository, which isn't set up yet) — for now, download and install the file per-release.

## Build from source

Requires **Go 1.21+**.

```bash
git clone https://github.com/AkshatRauthan/ScrollShot.git
cd ScrollShot

# Linux / macOS
go build -o scrollshot ./cmd/scrollshot

# Windows (cross-compile from Linux)
GOOS=windows GOARCH=amd64 go build -o scrollshot.exe ./cmd/scrollshot/
```

## Adding to PATH (run `scrollshot` from any terminal)

Skip this if you installed via `.deb`/`.rpm` — the package already puts the binary on your `PATH`.

### Linux / macOS

**Option A — copy to `/usr/local/bin` (system-wide):**

```bash
sudo cp scrollshot /usr/local/bin/
# Verify:
scrollshot --help
```

**Option B — copy to `~/.local/bin` (current user only, no sudo):**

```bash
mkdir -p ~/.local/bin
cp scrollshot ~/.local/bin/

# Make sure ~/.local/bin is on your PATH (add to ~/.bashrc or ~/.zshrc if missing):
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Verify:
scrollshot --help
```

### Windows

**Option A — move the binary to a permanent folder, then add it to PATH via GUI:**

1. Move `scrollshot.exe` to a stable folder, e.g. `C:\Tools\scrollshot\scrollshot.exe`.
2. Open **Start** → search `Environment Variables` → click **Edit the system environment variables**.
3. Under **System variables**, select **Path** → click **Edit** → click **New**.
4. Paste `C:\Tools\scrollshot` and click **OK** on all dialogs.
5. Open a new `cmd` or PowerShell window and verify:

```cmd
scrollshot --help
```

**Option B — one-liner via PowerShell (adds to your User PATH permanently):**

```powershell
# Run once in PowerShell (no admin required for user-level PATH)
$target = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force -Path $target | Out-Null
Copy-Item .\scrollshot.exe $target

$current = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($current -notlike "*$target*") {
    [Environment]::SetEnvironmentVariable("PATH", "$current;$target", "User")
    Write-Host "Added $target to PATH. Open a new terminal to use scrollshot."
}
```

## Linux: `/dev/uinput` permissions (required for `auto`)

The `auto` command injects scroll events through the Linux kernel's `uinput` module. That requires read/write access to `/dev/uinput`.

**One-time setup (recommended):**

```bash
# Add yourself to the input group
sudo usermod -aG input $USER

# Create a udev rule granting the input group access
echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-scrollshot.rules
sudo udevadm control --reload-rules && sudo udevadm trigger

# Log out and back in, then verify
ls -la /dev/uinput   # should show: crw-rw---- ... input input /dev/uinput
```

---

Installed and ready? Head to [`docs/usage.md`](usage.md) for commands, flags, and typical workflows.