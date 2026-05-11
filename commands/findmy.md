---
description: Look up Find My friend locations on macOS (name, location, staleness, distance) via Vision OCR of FindMy.app
argument-hint: "[<name>]"
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/findmy.sh:*)
---

# /findmy — Where is everyone?

Query Find My via the bundled `findmy` CLI. With no arguments, returns
every friend in the FindMy.app People sidebar. With a name argument,
returns just that friend (case-insensitive substring match).

## Run

If `$ARGUMENTS` is empty:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/findmy.sh" people --json
```

Otherwise:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/findmy.sh" person "$ARGUMENTS" --json
```

## How to report results to the user

- Lead with the friend's **name**, **location**, and **distance** when known.
- If `staleness` is `"Paused"`, **say so up front** — the location is the
  last known position, not live. Example: "Omar paused location sharing;
  last known position was Redmond, WA (7 mi away)."
- If `staleness` is a time string like `"5 min. ago"`, mention it
  inline: "Omar is in Redmond, WA — updated 5 min ago."
- If `staleness` is empty/missing, treat the location as live.
- For multi-person output, sort by name and present as a short list.
- For `person <name>` lookups that return "no person matching", say so
  plainly and offer to list everyone with `/findmy` (no args).

## Caveats to surface only when relevant

- This raises FindMy.app to the front briefly during the lookup.
- On a headless Mac, the display must be awake (the CLI nudges it via
  `caffeinate -u`, but a real or dummy USB-C display must be attached).
- Two `/findmy` invocations within ~5s can fail with "could not create
  image from window" — wait a few seconds and retry.
- The CLI reads UI text, not coordinates. No lat/lon is available.
