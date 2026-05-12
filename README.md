# findmy-cli

Read your Find My friends' locations from the macOS FindMy.app via UI scraping.
Apple does not expose a public API for friend locations and the on-disk caches
are encrypted with keychain-bound keys, so this tool drives the GUI: it
activates FindMy.app, switches to the People tab, screenshots the window, and
runs Vision OCR on the result.

## Why a Go CLI plus a Swift helper

The macOS APIs we need (Vision, CoreGraphics window list, CGEvent click) have no
Go binding. We bundle a tiny Swift binary `findmy-helper` that exposes them as
JSON-emitting subcommands, and a Go CLI `findmy` that orchestrates.

## Install

### Homebrew (recommended)

```bash
brew install omarshahine/tap/findmy-cli
```

Installs both `findmy` and `findmy-helper` to `/opt/homebrew/bin/`.

### OpenClaw plugin

```bash
clawhub install findmy-cli
```

The plugin shells out to the `findmy` binary — install that via Homebrew
first. Registers `findmy_people` and `findmy_person` tools.

### Build from source

```bash
make
```

Outputs `bin/findmy` and `bin/findmy-helper`.

Requirements:
- macOS (tested on 15+; FindMy.app is a Catalyst app)
- Go 1.22+
- Xcode Command Line Tools (`swiftc`)

## Usage

```
# List people in the sidebar with coarse location, staleness, distance.
findmy people
findmy people --json

# Click a row and OCR the detail pane (precise address).
findmy person "Omar Shahine"
findmy person "Omar Shahine" --json
```

## Required macOS permissions

Grant to the terminal emulator (or to `findmy` once installed system-wide):

- **Screen Recording** — for `screencapture`
- **Accessibility** — for `osascript` menu clicks

Settings → Privacy & Security → Screen Recording / Accessibility.

After granting, **fully quit and relaunch the host process** — TCC is read once
at process start.

## Limitations

- **The display must be awake and unlocked.** WindowServer stops compositing
  when the display sleeps, so `screencapture` returns a 99 KB all-black PNG
  even when the process itself runs fine. The CLI detects this and tells you
  to wake the keyboard. There is no software-only path to wake a sleeping
  display from userland — Apple gates `IODisplayWranglerWakeup` behind real
  HID hardware. For headless use, run `caffeinate -d` as a LaunchAgent or
  `pmset -a displaysleep 0`.
- The MapKit map area does not always render into the captured bitmap (Catalyst
  quirk). This tool only reads the sidebar and detail pane text; map pins are
  not extracted.
- The FindMy.app window must be openable on this Mac (you must be signed in to
  iCloud and have at least one friend sharing).
- Window position is re-queried on every run; the app does not need to be at a
  fixed location.
- This brings FindMy.app to the foreground and steals focus during a click.
- Apple's TOS may consider GUI scraping out of scope. Use at your own risk.

## Layout

```
cmd/findmy/                     Go CLI
internal/findmy/                Orchestration + sidebar parser
helpers/findmy-helper/main.swift  window + ocr + click subcommands
bin/                            Build outputs
.claude-plugin/plugin.json      Claude Code / OpenClaw plugin manifest
commands/findmy.md              /findmy slash command
skills/findmy/SKILL.md          Auto-triggering skill
scripts/findmy.sh               Plugin wrapper (auto-builds on first use)
```

## Plugin surfaces

The repo ships three plugin surfaces, all of which shell out to the same Go CLI:

| Surface | Manifest | Install |
|---|---|---|
| OpenClaw (NPM + ClawHub) | `openclaw/openclaw.plugin.json` | `clawhub install findmy-cli` |
| Claude Code (bundle) | `.claude-plugin/plugin.json` | `openclaw plugins install --link ~/GitHub/findmy-cli` |
| Homebrew CLI | `Formula/findmy-cli.rb` (omarshahine/homebrew-tap) | `brew install omarshahine/tap/findmy-cli` |

The Claude Code wrapper (`scripts/findmy.sh`) builds `bin/findmy` and
`bin/findmy-helper` on first invocation via `make`. No binaries are
committed.
