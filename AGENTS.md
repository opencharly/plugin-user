# AGENTS.md — plugin-user

Standalone plugin repo for the `user` multi-role state-provision verb
(`verb:user`). The plugin is a Go module at `candy/plugin-user/` (module path
`github.com/opencharly/plugin-user/candy/plugin-user`); the root `charly.yml`
only declares `discover: candy` so the repo is a project and its candy is
scanned.

Canonical files:

- `candy/plugin-user/charly.yml` — the `plugin-user:` candy entity (`plugin:`
  block, `plan:` check).
- `candy/plugin-user/plugin.go` — the verb implementation (`CheckVerbProvider` +
  `ProvisionActor`) + `NewMeta()`.
- `candy/plugin-user/schema/user.cue` — the self-contained `#UserInput`.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the unified Provider model, the multi-role state-provision contract,
  the per-plugin CUE-schema contract, placement. Load before touching the
  provider or schema.
- `/charly-core:service` — the sibling init-agnostic service-provision verb.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-user/` — compile the plugin module.
- `go test ./...` in `candy/plugin-user/` — the plugin's Go tests
  (`plugin_test.go`, `groups_test.go`).
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- The `plan:` check probes `root` via `getent passwd` on a live deployment.

## Modify this repo

- Edit the `plugin-user:` candy entity, the Go source, and `schema/user.cue`
  **together** — the schema is the single source for the `params/` struct, so a
  field change not mirrored in the schema desyncs the generated types.
- This plugin is **compiled-in only** (a host-coupled kit verb); do not describe
  it as out-of-process.

## Landing

- PR-only. Every change lands through a pull request; the org-required
  `charly/pr-validator` validates the diff and body and arms native auto-merge on
  PASS. Direct pushes to `main` are blocked.
- History lives in `CHANGELOG/` (written by `tag-on-merge` at merge time); the PR
  body IS the changelog.
- The authoritative rulebook is the umbrella `AGENTS.md` in
  `opencharly/opencharly` and `charly/AGENTS.md` in the charly repo. Do not
  restate its rules here.
