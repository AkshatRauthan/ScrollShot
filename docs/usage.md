# Usage Guide

A practical walkthrough of every command and flag — what to run, when, and why.

---

## Installation

### Pre-built binary

Download the latest release binary from the [Releases page](https://github.com/AkshatRauthan/ScrollShot/releases) and place it somewhere on your `PATH`.

### Build from source

Requires **Go 1.21+**.

```bash
git clone https://github.com/AkshatRauthan/ScrollShot.git
cd ScrollShot

# Linux / macOS
go build -o scrollshot ./cmd/scrollshot

# Windows (cross-compile from Linux)
GOOS=windows GOARCH=amd64 go build -o scrollshot.exe ./cmd/scrollshot/
```

### Adding to PATH (run `scrollshot` from any terminal)

#### Linux / macOS

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

#### Windows

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



### Linux: `/dev/uinput` permissions (required for `auto`)

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

**Quick alternative (no permanent setup):**

```bash
sudo scrollshot auto
```

---

## Commands

### `scrollshot capture`

Captures a single frame of the currently focused window and stores it in the session directory.

```
scrollshot capture [--debug] [-wait N]
```

**Workflow:**
1. Switch to the window you want to screenshot.
2. Run `scrollshot capture` (or set up a keyboard shortcut).
3. Scroll down a bit.
4. Run `scrollshot capture` again.
5. Repeat until you've covered everything.
6. Run `scrollshot finish` to stitch and save.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `-wait N` | `5` | Wait N seconds before capturing (gives you time to switch windows) |
| `--debug` | off | Print verbose logs showing which backend is used, paths, frame dimensions |

**Example:**

```bash
# Capture with 3-second delay (enough time to click the target window)
scrollshot capture -wait 3
```

---

### `scrollshot auto`

The one-command workflow: scrolls the active window automatically, captures each scroll step, and stitches everything at the end.

```
scrollshot auto [--debug] [-wait N]
```

**Workflow:**
1. Switch to the window you want to screenshot.
2. Run `scrollshot auto` — it waits, then scrolls and captures automatically.
3. When scrolling reaches the bottom (or stagnates), it stitches and saves the final image.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `-wait N` | `5` | Wait N seconds before starting (gives you time to switch windows) |
| `--debug` | off | Print per-frame logs: advance pixels, overlap score, stagnation count |

**Example:**

```bash
# Start auto-scroll immediately (no wait)
scrollshot auto -wait 0

# Debug a problem with autoscroll stopping too early
scrollshot auto --debug
```

**How it stops:**
- Reached the maximum frame count (default: 50).
- Three consecutive frames where scroll advance is below the minimum pixel threshold (page bottom detected).
- Three consecutive frames where no confident overlap match was found (highly animated or non-scrollable content).

---

### `scrollshot finish`

Stitches all captured frames from the current session into a single image and saves it.

```
scrollshot finish [--debug]
```

Use this after a manual `capture` session. `auto` calls this automatically, so you don't need it after `auto`.

**Output location** (tried in order, first writable wins):

| Platform | Location |
|---|---|
| Linux / macOS | `~/Pictures/Screenshots/scrolling_<timestamp>.png` |
| Windows | `%USERPROFILE%\Pictures\Screenshots\scrolling_<timestamp>.png` |
| Fallback (any) | `~/Pictures/` or `~/scrollshot_output/` |

**Example:**

```bash
scrollshot finish --debug
# [stitch] stitching 7 frames...
# [stitch] frame 1: overlap 312px, added 415px
# ...
# Saved: /home/user/Pictures/Screenshots/scrolling_20260805_220000.png
```

---

### `scrollshot version`

Prints the build version.

```bash
scrollshot version
```

---

### `scrollshot --help` / `-h`

Prints usage for any command.

```bash
scrollshot --help
scrollshot capture --help
scrollshot auto -h
```

---

## Global Flags

| Flag | Description |
|---|---|
| `--debug` | Enable verbose output for any command. Equivalent to `SCROLLSHOT_DEBUG=1`. |

You can also enable debug mode via environment variable — useful when debugging from a shortcut or script:

```bash
SCROLLSHOT_DEBUG=1 scrollshot auto
```

---

## Typical Workflows

### Manual: long web page

```bash
# 1. Open the page in your browser
# 2. Capture the top
scrollshot capture -wait 3

# 3. Scroll down a screenful, capture again
scrollshot capture -wait 3
# (repeat 4–6 times)

# 4. Stitch
scrollshot finish
```

### Automatic: any scrollable window

```bash
# Click the window you want, then immediately:
scrollshot auto
# It waits 5s, scrolls, captures, and saves — fully hands-free
```

### Debugging a capture problem

```bash
scrollshot capture --debug
# Shows: which backend was chosen, session dir path, frame dimensions
# If it fails: shows exactly which backend's Available() returned false and why
```

---

## Where Files Are Stored

| Purpose | Linux | Windows |
|---|---|---|
| Session frames | `~/.cache/scrollshot_session/` | `%LocalAppData%\scrollshot_session\` |
| Final output | `~/Pictures/Screenshots/` | `%USERPROFILE%\Pictures\Screenshots\` |

Session frames are automatically cleared after a successful `finish` or `auto`. If a session is stale (older than 2 minutes), it's also automatically cleared at the start of the next `capture`.

---

## Troubleshooting

### `auto` fails with "no supported scrolling backend"

**Linux:** You don't have access to `/dev/uinput`. See [Linux uinput permissions](#linux-devinput-permissions-required-for-auto) above.

**Other platform:** `auto` is not yet supported on macOS.

### `capture` fails with "no capture backend available"

Your desktop environment isn't supported by any of the auto-detected backends. Run with `--debug` to see what was tried:

```bash
scrollshot capture --debug
```

Check [`docs/capturing.md`](capturing.md) for the full backend list and what each one requires.

### `finish` fails with "could not save output"

On Windows, this is almost always a **Controlled Folder Access** (ransomware protection) or **OneDrive** issue blocking writes to `Pictures`. Scrollshot tries multiple fallback locations automatically — run with `--debug` to see which ones were tried:

```bash
scrollshot finish --debug
# [paths] candidate C:\Users\HP\Pictures\Screenshots failed os.Create: ...
# [paths] candidate C:\Users\HP\Pictures failed os.Create: ...
# [paths] successfully created output file: C:\Users\HP\scrollshot_output\...
```

If all fallbacks fail, the quickest fix is to add `scrollshot.exe` to the **Controlled Folder Access** allow-list in Windows Security settings.

### The stitched image has a visible seam or duplicate content

Run with `--debug` and check the overlap scores:

```bash
scrollshot finish --debug
# Frame 3: matched 280px overlap, added 390px  ⚠ no confident match found (best score 41%)
```

A low score on a frame means the stitcher couldn't find a good overlap — usually because:
- You scrolled too far between captures (overlap shrank below the match window).
- The content is highly repetitive (identical-looking rows).
- Animated content changed between captures.

Try recapturing with smaller scroll steps.
