---
name: findmy
description: |
  Query Find My friend locations on macOS via the findmy-cli plugin tools.
  Returns name, coarse location, staleness, and distance for everyone in
  the FindMy.app People sidebar. Use when the user asks "where is X", "is
  X home", "how far is X", or wants a location refresh.
---

# Find My Location Query

Two tools available, both shell out to the `findmy` binary which drives
FindMy.app via screen capture and Vision OCR.

## When to use

- "Where is Omar?"        → call `findmy_person` with `name: "Omar"`
- "Is Sarah home yet?"    → call `findmy_person` with `name: "Sarah"`
- "How far away is Mike?" → call `findmy_person` with `name: "Mike"`
- "Anyone near downtown?" → call `findmy_people`, then read the locations
- "Where is everyone?"    → call `findmy_people`

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

- `name` — display name as shown in the FindMy sidebar
- `location` — city, state (or device label when sharing from a device)
- `staleness` — `"Now"`, `"X min. ago"`, `"X hr. ago"`, `"Paused"`, `""` (live)
- `distance` — distance from this Mac if FindMy shows it (e.g. `"7 mi"`)

## Caveats to surface to the user

- **`staleness: "Paused"`** means the friend paused location sharing. The
  reported location is the last known position, possibly hours or days old.
  Lead with this when reporting the result.
- **Stale staleness** (`"7 hr. ago"`, etc.) means the device hasn't checked
  in recently — phone may be off, in low-power mode, or out of signal.
- **Focus steal**: each invocation briefly raises FindMy.app to the front.
- **Back-to-back races**: two findmy calls within ~5s can fail. Space them
  out when iterating.

## Install requirement

The plugin shells out to the `findmy` binary. If a tool returns
`"findmy not found on PATH"`, the binary isn't installed. Install with:

```bash
brew install omarshahine/tap/findmy-cli
```

After install, grant **Screen Recording** to the host process running this
plugin (System Settings → Privacy & Security → Screen Recording). Without
it, FindMy.app captures will return blank.
