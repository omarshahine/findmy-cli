## Clawpatch Code Review

This repo uses [Clawpatch](https://clawpatch.ai) for local automated code review. Keep `.clawpatch/` ignored; it is generated runtime state containing features, findings, reports, runs, and patch attempts.

Standard workflow:

```bash
clawpatch doctor
clawpatch init          # first time only
clawpatch map
clawpatch review --limit 10
clawpatch report --output .clawpatch/reports/summary.md
clawpatch show --finding <id>
clawpatch fix --finding <id>
clawpatch revalidate --finding <id>
```

If this repo needs hand-authored feature coverage, keep those curated definitions in `tools/clawpatch/features/` and sync/copy them into `.clawpatch/features/` before review. Do not commit `.clawpatch/` generated state.


<!-- BEGIN CLAUDE MEMORY IMPORT: -Users-omarshahine-GitHub-findmy-cli -->
## Imported Claude Project Memory

Durable memory promoted from `~/.claude/projects/-Users-omarshahine-GitHub-findmy-cli/memory` during the AGENTS.md migration. Keep this section current when project-specific operating knowledge changes.

### memory/MEMORY.md

- [Homebrew tap auto-bump](homebrew-tap-autobump.md) — 6 formulas self-update on v* tag via per-repo workflow + HOMEBREW_TAP_TOKEN (in chezmoi)

### memory/homebrew-tap-autobump.md

---
name: homebrew-tap-autobump
description: How omarshahine/homebrew-tap formulas auto-update on release
metadata: 
  node_type: memory
  type: project
  originSessionId: be5786e9-1fd7-4b4a-9486-e20512188e64
---

`omarshahine/homebrew-tap` (cloned at `~/GitHub/homebrew-tap`) holds 6 formulas, each built from the source tarball (no bottles):
- findmy-cli (Go+Swift), aqara-cli (Python), lutron-cli (Python), apple-pim-cli (Swift), daikin-cli (Go), trakt-cli (Go)

Each source repo has `.github/workflows/publish-homebrew.yml` that, on every `v*` tag, computes the GitHub source tarball sha256 and pushes a `url`+`sha256` bump to the tap. The workflow only updates the first app `url`/`sha256` pair, so vendored resource blocks (Python deps in aqara/lutron, swift-argument-parser in apple-pim) are left untouched — correct, since those change independently of app version.

Gated on repo secret `HOMEBREW_TAP_TOKEN`: a fine-grained PAT scoped ONLY to `homebrew-tap` with Contents: read/write (the default GITHUB_TOKEN can't push cross-repo). Stored in chezmoi as `HOMEBREW_TAP_TOKEN` in `~/.secrets-macbook-pro.env`. The SAME token value is set as a secret on all 6 source repos — reusable because it grants access to the tap, not the source repo.

To add a new project to the tap: write the formula, copy `publish-homebrew.yml` (set `FORMULA_PATH` to the formula name, which may differ from repo name — e.g. repo `trakt-plugin` → `Formula/trakt-cli.rb`, repo `Apple-PIM-Agent-Plugin` → `Formula/apple-pim-cli.rb`), then `printf '%s' "$HOMEBREW_TAP_TOKEN" | gh secret set HOMEBREW_TAP_TOKEN --repo omarshahine/<repo>` after sourcing the env.

apple-pim-cli formula note: Homebrew's build sandbox blocks network, so the formula vendors swift-argument-parser as a `resource` and `inreplace`s Package.swift to a local `.package(path:)` for an offline `swift build`. Verified to compile all 4 binaries (calendar/reminder/contacts/mail-cli).

<!-- END CLAUDE MEMORY IMPORT: -Users-omarshahine-GitHub-findmy-cli -->
