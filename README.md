# Scrollshot

**Native scrolling screenshots for Linux — built because nothing else worked.**

Scrollshot captures long, scrollable content — web pages, design canvases, code editors, chat logs, documentation — as a single continuous image, without relying on `wlr-layer-shell`-based tools (`grim`, `slurp`) that silently fail on GNOME/Mutter and other non-wlroots Wayland compositors.

It exists because every existing scrolling-screenshot tool on Linux assumes a compositor protocol that GNOME deliberately doesn't implement. Scrollshot works around that by capturing through the desktop's own native screenshot mechanism instead, then stitching frames together with a pixel-overlap matching engine — no compositor cooperation required beyond what already works.

---

## Why this matters

### For agentic coding
AI coding agents increasingly need to *see* what they're building — a rendered web app, a long settings page, a multi-screen user flow, a scrolling terminal log. A single viewport screenshot only shows a fraction of the actual UI state. Scrollshot gives an agent (or the human directing one) a complete, continuous view of a page or window in one image — critical for visual QA loops, regression comparison, and grounding an agent's understanding of what it actually shipped, instead of guessing from a cropped viewport.

### For UI/UX design
Full-page mockups, design systems, and long user flows are painful to document with viewport-limited tools. Scrollshot captures an entire scrollable canvas — a Figma board, a live prototype, a full page of components — in one clean image, ready to drop into a spec doc, a handoff file, or a design review without stitching screenshots by hand.

### For everything else
- **QA & bug reports** — capture an entire failing page state, not just what fit on screen
- **Documentation & tutorials** — full-page reference captures without seams
- **Code review** — long files, long diffs, long terminal output, captured whole
- **Research & archiving** — long articles, threads, and chat logs preserved completely
- **Support & compliance** — full-context evidence capture in one artifact instead of a stitched-together folder of overlapping crops

---

## How it works

1. **Capture** — each keypress (or command) grabs the currently focused window via the desktop's own screenshot mechanism (currently `gnome-screenshot`, which talks to GNOME Shell directly — no wlroots protocol dependency).
2. **Stack frames** — you scroll a bit between captures; each frame naturally overlaps the previous one.
3. **Stitch** — a pixel-overlap search finds exactly where consecutive frames match, trims the duplicate region, and glues them into one continuous image. No manual alignment, no visible seams.
4. **Save** — the finished image lands in `~/Pictures/Screenshots/`, ready to use.

Verified on real-world captures up to 23 frames at 2.8K resolution with zero visible seams.

---

## Status

Currently working on **Linux, GNOME, Wayland** — the hardest environment to support, tackled first since it's the one every other tool gives up on.

Built in Go with a deliberately modular architecture so new platforms and features are additive, not rewrites:

```
scrollshot/
├── cmd/scrollshot/         CLI entrypoint
└── internal/
    ├── capture/            pluggable screenshot backends (per OS/compositor)
    ├── session/            frame staging between capture and finish
    ├── stitch/             the overlap-detection stitching engine
    ├── edit/               [planned] crop / reorder frames
    ├── export/             [planned] lossless & lossy size control
    └── autoscroll/         [planned] driven scrolling + end-of-page detection
```

Each capture backend is just an implementation of a small `Capturer` interface — adding a new platform never touches the stitching logic, the CLI, or any other backend.

---

## Roadmap

**Cross-platform support** is the near-term priority — not just Ubuntu/GNOME, but Linux broadly and beyond:

- **Wayland, any desktop** — a direct `org.freedesktop.portal.Screenshot` D-Bus client, removing the GNOME-only / `gnome-screenshot`-installed dependency and extending native support to KDE and other portal-compliant desktops
- **X11** — any Linux distro on X11, via native capture, no external binary dependency
- **macOS** — native capture via CoreGraphics
- **Windows** — native capture with active-window targeting and DPI-awareness handling
- **Automatic backend detection** — the right capture method picked at runtime, no manual configuration

**Planned features:**
- **Autoscroll + auto-capture** — drive the scrolling automatically and capture continuously until the end of the page or a manual stop, instead of manual scroll-and-tap
- **Frame cropping** — trim sticky headers/nav bars that get recaptured in every frame before stitching
- **Frame reordering** — fix a session captured out of sequence before it's stitched
- **Lossless & lossy export control** — tighter compression for exact-pixel needs, or scaled-down/re-encoded output when file size matters more than fidelity

---

## Community

This started as a fix for a problem GNOME Wayland users have been hitting for years with no clean answer. If it's useful to you too:

- **Try it, break it, file an issue** — especially on desktop environments and distros beyond GNOME/Ubuntu; real-world edge cases are how the capture backends and overlap matching get more robust
- **Contribute a backend** — the `Capturer` interface is small and self-contained; a KDE, X11, macOS, or Windows backend is a well-scoped, independent contribution
- **Share how you're using it** — agentic workflows, design handoffs, QA pipelines, anything — it shapes what gets prioritized next

No roadmap here is fixed in stone. If a platform or feature above matters to your workflow, say so — that's exactly the kind of signal that reorders priorities.

---