---
name: findmy
description: |
  Query Find My friend locations on macOS via the findmy-cli plugin tools.
  Returns name, coarse location (city, state), staleness, and distance for
  everyone in the FindMy.app People sidebar. Use when the user asks "where
  is X", "is X home", "how far is X", or wants a location refresh.
license: MIT
metadata:
  author: Omar Shahine
  version: 0.2.0
  openclaw:
    emoji: pushpin
    os: [darwin]
    homepage: https://github.com/omarshahine/findmy-cli
    requires:
      bins: [findmy, findmy-helper]
    install:
      - kind: brew
        id: findmy-cli
        label: "Install findmy and findmy-helper via Homebrew"
        formula: omarshahine/tap/findmy-cli
        bins: [findmy, findmy-helper]
---

# Find My Skill

Two tools available, both shell out to the `findmy` binary which drives
FindMy.app via screen capture and Vision OCR. Read-only — never mutates
FindMy.app state.

## When to use

- "Where is Omar?"        → call `findmy_person` with `name: "Omar"`
- "Is Sarah home yet?"    → call `findmy_person` with `name: "Sarah"`
- "How far away is Mike?" → call `findmy_person` with `name: "Mike"`
- "Anyone near downtown?" → call `findmy_people`, then read the locations
- "Where is everyone?"    → call `findmy_people`
- "Where am I?"           → call `findmy_person` with `name: "Me"`. The first
                            entry in the FindMy People sidebar is always the
                            owner of this Mac, labeled `"Me"`. Same shape as
                            other entries but with no `staleness` or `distance`.

## Using location to drive other actions

Location is high-trust data — knowing where someone is can unlock useful
follow-ups: "you're near Pike Place, want me to book a table at Matt's?",
"Sarah's still 7 mi away, push the dinner reservation by 30 min", "you're
home, run the arrival routine."

Before chaining a location result into a mutating action (booking, sending
a message, triggering a routine, ordering something), **ask the user for
explicit approval first**. State the location you used, the action you'd
take, and wait for confirmation. Examples:

- ✅ "You're in Seattle near downtown. Want me to search OpenTable for
  dinner reservations within 1 mile?"
- ✅ "Sarah is 15 min away. Should I text her the parking instructions?"
- ❌ Calling `restaurant_book` based purely on the inferred location, no
  confirmation step.

Read-only location queries themselves never need approval — those are what
the user asked for. The approval gate is on the *next* tool call that
turns location into an action.

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

## ClawScan note

This skill drives FindMy.app by raising it to the foreground, capturing a
screenshot of its window, running Apple's Vision OCR on the image, and
parsing the resulting text. The behavior may look unusual to a static
scanner — screen capture, OCR, and UI scraping — but it is the only path
to friend location data, since Apple does not expose this through any
public API. The plugin does not click, type into, or otherwise mutate
FindMy.app; it is read-only. No network traffic is initiated by this
plugin. All data stays on-device.
