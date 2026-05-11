---
name: findmy
description: |
  Query Find My friend locations on macOS. Returns name, coarse location,
  staleness, and distance for everyone in the FindMy.app People sidebar.
  Use when the user asks "where is X", "is X home", "how far is X", or
  wants a location refresh for a friend. macOS only; requires the
  display to be awake (the skill self-wakes via caffeinate) and Screen
  Recording granted to the host process.
argument-hint: "[people | person <name>] [--json]"
---

# /findmy — Find My Location Query

Wraps the `findmy` CLI bundled with this plugin. The CLI drives FindMy.app:
activates it, switches to the People tab, screencaptures the window, runs
Vision OCR, and parses the sidebar.

## When to use

- "Where is Omar?"
- "Is Sarah home yet?"
- "How far away is Mike?"
- "Anyone near downtown?"

## Run

```bash
# List everyone in the sidebar (default: human-readable table)
bash "${CLAUDE_PLUGIN_ROOT}/scripts/findmy.sh" people

# Same, as JSON — best for programmatic follow-up
bash "${CLAUDE_PLUGIN_ROOT}/scripts/findmy.sh" people --json

# Lookup a single friend (name match is case-insensitive, substring OK)
bash "${CLAUDE_PLUGIN_ROOT}/scripts/findmy.sh" person "Omar Shahine" --json
```

The wrapper auto-builds `bin/findmy` and `bin/findmy-helper` on first run via
`make` (needs Go 1.22+ and Xcode Command Line Tools). Subsequent runs just
exec the binary.

## Output shape (JSON)

```json
[
  {
    "name": "Omar Shahine",
    "location": "Redmond, WA",
    "staleness": "Paused",
    "distance": "7 mi"
  }
]
```

- `name` — display name as shown in the FindMy sidebar
- `location` — city, state (or device label when sharing from a device)
- `staleness` — `"Now"`, `"X min. ago"`, `"X hr. ago"`, `"Paused"`, `""` (live)
- `distance` — distance from this Mac if FindMy shows it (e.g. `"7 mi"`, `"1,971 mi"`)

## Caveats — surface these when relevant

- **`staleness: "Paused"`** means the friend has paused location sharing. The
  reported location is the last known position, possibly hours or days old.
  Lead with this when reporting the result.
- **Display sleep**: the CLI wakes the display via `caffeinate -u -t 3`
  before each capture. If running on a truly headless Mac, ensure a display
  (real or dummy USB-C plug) is attached — FindMy.app needs WindowServer
  compositing.
- **Focus steal**: each invocation briefly raises FindMy.app to the front.
- **Back-to-back races**: two `findmy` invocations within ~5s can fail with
  "could not create image from window" — space them out.
- **No coordinates**: this is OCR of the sidebar text; lat/lon is not
  available. Apple doesn't expose friend locations through any public API.

## Permission requirements (one-time)

Grant to the terminal emulator (or to the host process running this skill):

- **Screen Recording** — System Settings → Privacy & Security → Screen Recording
- **Accessibility** — only needed if a future version starts clicking rows
  (not used today)

After granting, **fully quit and relaunch** the host — TCC is read once at
process start. The CLI's `findmy-helper permissions` subcommand can verify:

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/findmy-helper" permissions
# → {"accessibility":true,"screenRecording":true}
```
