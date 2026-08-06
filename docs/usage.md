# Usage Guide

A practical walkthrough of every command and flag — what to run, when, and why.

---

## Before you start

New here? Installation (pre-built binary, `.deb`/`.rpm`, build from source, adding to `PATH`, and Linux `/dev/uinput` permissions) now lives in **[`docs/installation.md`](installation.md)**. This guide assumes `scrollshot` is already installed and on your `PATH`.

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

**Linux:** You don't have access to `/dev/uinput`. See [Linux uinput permissions](installation.md#linux-devinput-permissions-required-for-auto) in the installation guide.

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
