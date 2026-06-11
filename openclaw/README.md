# Find My

OpenClaw plugin for macOS Find My friend locations. It shells out to the `findmy` binary, which drives FindMy.app via screen capture and Apple's Vision OCR, then returns each person's name, coarse location, staleness, and distance. Read-only: it never mutates FindMy.app state.

**macOS only.** Requires Screen Recording permission granted to the host process.

## Privacy & consent

Location is sensitive, so the boundaries here are deliberate:

- **Opt-in only.** This plugin can read *only* the people who have already chosen
  to share their location with this Mac's Apple ID in Apple's Find My. Sharing is
  a mutual relationship that the other person controls and can revoke at any time.
  There is no way for this plugin to see anyone who has not opted in, and it does
  not bypass, weaken, or work around any of Apple's access controls.
- **Coarse data only.** It returns the same city/state, staleness, and distance
  that FindMy.app already shows the signed-in user — no precise coordinates, no
  location history, no background tracking.
- **On-device.** It initiates no network traffic. Nothing leaves your Mac.
- **Authorized use.** It is meant for the owner of this Mac to locate friends and
  family who are knowingly sharing with them — coordinating a pickup, checking an
  ETA, running an arrival routine. It is **not** a surveillance tool. Do not use it
  to monitor or track anyone without their knowledge and consent.

Run this on a Mac you control. Avoid wiring it into shared or unattended agent
setups where someone other than the account owner could query a friend's location.

## Install

1. Install the plugin from ClawHub.
2. Install the CLI it depends on:

   ```bash
   brew install omarshahine/tap/findmy-cli
   ```

3. Grant **Screen Recording** to the host process running this plugin
   (System Settings → Privacy & Security → Screen Recording). Without it,
   FindMy.app captures come back blank.

## Tools

| Tool | Description |
|------|-------------|
| `findmy_person` | Locate one person by name (`name: "Omar"`). Use `name: "Me"` for this Mac's owner. |
| `findmy_people` | Locate everyone in the FindMy People sidebar. |

Both are read-only and consent-bounded — they can only return people who are
already sharing with this Mac's Apple ID (see [Privacy & consent](#privacy--consent)).
The lookup itself answers a question the account owner asked. The approval gate
belongs on whatever action you chain *after* a location result (booking,
messaging, triggering a routine): state the location you used and get explicit
confirmation before acting on it.

## Output shape

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

- `name` — display name from the FindMy sidebar
- `location` — city, state (or device label when sharing from a device)
- `staleness` — `"Now"`, `"X min. ago"`, `"X hr. ago"`, `"Paused"`, or `""` (live)
- `distance` — distance from this Mac if FindMy shows it

## Configuration

| Key | Default | Description |
|-----|---------|-------------|
| `cliPath` | `findmy` | Path or command name for the `findmy` binary (PATH lookup by default) |

## Caveats

- **`staleness: "Paused"`** means the friend paused sharing. The location is the last known position, possibly hours or days old. Lead with this when reporting.
- **Stale timestamps** (`"7 hr. ago"`) mean the device has not checked in recently (phone off, low-power mode, or no signal).
- **Focus steal**: each call briefly raises FindMy.app to the foreground.
- **Back-to-back races**: two calls within ~5 seconds can fail. Space them out.

## How it works (scanner note)

This plugin reads friend locations by raising FindMy.app to the foreground,
screenshotting its window, running Apple Vision OCR on the image, and parsing
the text. The behavior may look unusual to a static scanner (screen capture,
OCR, UI scraping), but it is the only path to friend location data since Apple
exposes no public API for it. The plugin does not click, type into, or mutate
FindMy.app, and it initiates no network traffic. All data stays on-device.

## License

MIT (c) Omar Shahine
