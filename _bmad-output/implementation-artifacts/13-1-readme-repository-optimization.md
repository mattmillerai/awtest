# Story 13.1: README & Repository Optimization

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a potential user discovering awtest on GitHub,
I want a clear, professional README that immediately communicates what the tool does and how to install it,
So that I can evaluate and start using it within minutes.

## Acceptance Criteria

1. The README contains: one-line description, key feature highlights, installation instructions, quick-start example, and output sample — and works as a standalone landing page
2. GitHub repository topics/tags include at minimum: `aws`, `security`, `enumeration`, `pentest`, `cloud-security`, `credential-testing`, `golang`, `cli`
3. The GitHub repository description is set to a clear, professional one-liner
4. The README accurately represents the tool's current capabilities (63 services, 117 API calls, concurrent scanning, macOS signed binaries)
5. The Go version badge reflects the actual minimum version (1.24, not 1.19)

## Tasks / Subtasks

- [x] Task 1: Update README.md content and structure (AC: #1, #4, #5)
  - [x] 1.1 Update Go version badge from `1.19+` to `1.24+`
  - [x] 1.2 Refine the one-line description/tagline for clarity and SEO (current: "AWS Credential Permission Scanner for Security Assessments")
  - [x] 1.3 Add a "What it does" or "Why AWTest" quick-summary section immediately after badges — 2-3 sentences explaining the tool's value proposition for the landing-page effect
  - [x] 1.4 Ensure the quick-start example is prominent and shows the simplest possible invocation (`awtest` with default profile)
  - [x] 1.5 Verify the output sample accurately reflects current output format
  - [x] 1.6 Add note about macOS code signing (no Gatekeeper warnings) in the Installation section under Homebrew
  - [x] 1.7 Add `--check-update` flag to the Flags table (added in Story 11.1 but may not be in README)
  - [x] 1.8 Update `go install` instructions from `Go 1.19+` to `Go 1.24+`
  - [x] 1.9 Review and tighten all sections for landing-page readability — remove redundancy, ensure scannability
- [x] Task 2: Set GitHub repository topics (AC: #2)
  - [x] 2.1 Set topics on public repo using: `gh repo edit MillerMedia/awtest --add-topic aws,security,enumeration,pentest,cloud-security,credential-testing,golang,cli`
  - [x] 2.2 Consider additional relevant topics: `aws-security`, `penetration-testing`, `red-team`, `cloud-enumeration`, `infosec`
- [x] Task 3: Update GitHub repository description (AC: #3)
  - [x] 3.1 Set description using: `gh repo edit MillerMedia/awtest --description "..."` with a clear one-liner
  - [x] 3.2 Suggested description: "Quickly enumerate AWS credential permissions across 63 services — built for pentesters, red teamers, and cloud security assessors"
- [x] Task 4: Verify changes render correctly (AC: #1)
  - [x] 4.1 Preview README rendering in GitHub (check badges, tables, collapsible sections, code blocks)
  - [x] 4.2 Confirm topics appear on the repository page
  - [x] 4.3 Confirm description appears under the repository name

## Dev Notes

### Architecture & Approach

This is a documentation and repository metadata story — no Go code changes required. All changes are to `README.md` (file) and GitHub repository settings (API calls via `gh` CLI).

**Key principle:** The README must work as a standalone landing page. A user landing from a Google search, awesome-list link, or conference talk should be able to understand, evaluate, and install the tool without clicking any other links.

### Current State Analysis

**README.md (current):**
- Already has: ASCII banner, badges, one-liner, feature bullets, installation (3 methods), usage examples (7 scenarios), flags table, speed presets explanation, output formats, 3 real-world use cases, full service list (collapsible), contributing section, support section, license
- Needs: Go version badge update (1.19 → 1.24), `--check-update` flag in table, macOS signing note, possible tagline refinement, landing-page flow optimization

**GitHub Repository (current):**
- Description: "Pentesting tool for AWS Credentials" — too terse, doesn't communicate breadth
- Topics: **NONE set** — zero discoverability through GitHub topic search
- These are set via `gh repo edit` on the PUBLIC repo (MillerMedia/awtest)

**Go version badge discrepancy:** The README badge says `1.19+` but `go.mod` was upgraded to `1.24` in Story 12.1. The `go install` section also says "Requires Go 1.19+". Both must be updated.

**`--check-update` flag:** Added in Story 11.1 (commit `487f508`). Verify if it's already in the flags table — if not, add it.

### Changes Required

| File/Target | Change | Why |
|---|---|---|
| `README.md` | Update Go version badge | Badge shows 1.19 but go.mod is 1.24 |
| `README.md` | Update `go install` prereq text | Same version discrepancy |
| `README.md` | Add `--check-update` to flags table (if missing) | Added in Story 11.1 |
| `README.md` | Add macOS signing note | Story 12.1 added signing |
| `README.md` | Refine tagline/one-liner | Landing page optimization |
| `README.md` | Tighten flow for landing-page readability | FR231 requirement |
| GitHub repo settings | Set 8+ topics | FR233 — zero topics currently |
| GitHub repo settings | Update description | AC #3 — current description too terse |

### What NOT to Do

1. **Do NOT change the ASCII banner** — the banner is fine and a rename is deferred to Epic 14
2. **Do NOT restructure the README dramatically** — the current structure is solid; this is optimization, not a rewrite
3. **Do NOT add a GIF/video/screenshot** — the tool is a CLI; the text output sample is sufficient
4. **Do NOT remove the Buy Me a Coffee section** — it's the maintainer's choice
5. **Do NOT set topics on the private repo** — only the PUBLIC repo (MillerMedia/awtest) matters for discoverability
6. **Do NOT change the repo name** — that's Epic 14 (deferred)
7. **Do NOT modify Go code** — this is a docs/metadata-only story

### Previous Story Intelligence (Story 12.1)

- Story 12.1 upgraded Go from 1.19 to 1.24 in `go.mod` — this means the README Go version references are now stale
- Story 12.1 added macOS code signing and notarization — the README should mention this as a feature (frictionless macOS install)
- The `gh` CLI was used for secret management in 12.1 — same tool will be used for repo settings here
- No Go code changes were needed in 12.1, and none are needed here either

### Git Intelligence

Recent commits show:
- `cb50b66` — Story 12.1: macOS code signing (Go version bumped)
- `f97390c` — Story 11.1: Non-blocking update check added to scan output
- `487f508` — Story 11.1: `--check-update` flag implementation
- `227d5d6` — Story 10.1: README was last comprehensively updated here (63 services, speed presets)

The README was last updated in Story 10.1. Stories 11.1 and 12.1 added features (update check, macOS signing) that may not be reflected in the README yet.

### Suggested GitHub Topics

Required (per AC): `aws`, `security`, `enumeration`, `pentest`, `cloud-security`, `credential-testing`, `golang`, `cli`

Recommended additions: `aws-security`, `penetration-testing`, `red-team`, `infosec`, `cloud-enumeration`

### Suggested Repository Description

Current: "Pentesting tool for AWS Credentials"

Proposed: "Quickly enumerate AWS credential permissions across 63 services — built for pentesters, red teamers, and cloud security assessors"

Alternative: "AWS credential permission scanner — 63 services, 117 API calls, concurrent scanning for pentesters and red teamers"

### Project Structure Notes

- Alignment with unified project structure: README.md is at project root, standard location
- No conflicts — this story modifies one file and sets GitHub API metadata
- The sync workflow (sync-public.yml) will push README changes to the public repo on next tag

### References

- [Source: _bmad-output/planning-artifacts/epics-phase3.md#Epic 13] — Story 13.1 AC and requirements
- [Source: _bmad-output/planning-artifacts/prd-phase3.md#FR230] — FR231 (README as landing page), FR233 (topics/tags)
- [Source: README.md] — Current README content (last major update: Story 10.1)
- [Source: _bmad-output/implementation-artifacts/12-1-apple-developer-certificate-setup-goreleaser-signing.md] — Go version bump, macOS signing context
- [Source: go.mod] — Go 1.24 (updated from 1.19 in Story 12.1)
- [GitHub CLI: gh repo edit](https://cli.github.com/manual/gh_repo_edit) — For setting topics and description
- [GitHub: About topics](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics) — Topic best practices

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- Verified output format by tracing `PrintResult` → `ColorizeMessage` in `cmd/awtest/utils/output.go` — all services pass `""` as moduleName (defaults to "AWTest")
- Confirmed `--check-update` flag defined in `cmd/awtest/main.go:58` but was missing from README flags table
- Verified Go version in `go.mod` is 1.24 (updated in Story 12.1)

### Completion Notes List

- **Task 1:** Updated README.md — Go badge 1.19→1.24, refined tagline to "Fast AWS Credential Permission Enumeration for Pentesters & Red Teamers", added "What It Does" section with value proposition, added Quick Start section, corrected example output to match actual code output format (added scan summary), added macOS signing note under Homebrew install, added `--check-update` flag to flags table, updated go install prereq to 1.24+, tightened Usage section by removing duplication with Quick Start, consolidated Features list (merged Speed Presets/Concurrent Scanning, added Signed macOS Binaries)
- **Task 2:** Set 13 GitHub topics on MillerMedia/awtest: aws, security, enumeration, pentest, cloud-security, credential-testing, golang, cli, aws-security, penetration-testing, red-team, infosec, cloud-enumeration
- **Task 3:** Updated GitHub repo description to: "Quickly enumerate AWS credential permissions across 63 services — built for pentesters, red teamers, and cloud security assessors"
- **Task 4:** Verified topics (13) and description appear correctly via `gh repo view`. README structure validated: badges, tables, collapsible details, code blocks all use standard GitHub-flavored markdown.

### File List

- `README.md` — Updated (Go badge, tagline, What It Does section, Quick Start, output example, macOS signing note, --check-update flag, go install version, tightened structure)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — Updated story status
- `_bmad-output/implementation-artifacts/13-1-readme-repository-optimization.md` — Updated task checkboxes, status, dev agent record

### Change Log

- 2026-03-15: Implemented Story 13.1 — README & Repository Optimization. Updated README.md for landing-page readability, corrected Go version references, added missing --check-update flag, added macOS signing note. Set 13 topics and updated description on public GitHub repo (MillerMedia/awtest).
