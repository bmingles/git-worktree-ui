# Load worktree git status asynchronously (progressive fill-in)

## Problem

Expanding a project node still feels slow. Even after parallelizing the per-worktree
`git status` calls (see archived `parallelize-worktree-status.md`), expansion blocks until
**all** statuses for the project finish before any worktree rows appear. On the Iris repo
(11 worktrees) that's ~2.8s of blank wait, despite `ListWorktrees` itself taking only ~17ms.

`git status` output drives **only three cosmetic trailing annotations** on a worktree row in
[view.go renderWorktree](../wt/pkg/tui/view.go#L376-L426):

- `*` (orange) — `Status.HasChanges`
- `↑N` (green) — `Status.AheadBy`
- `↓N` (red) — `Status.BehindBy`

The row itself (branch name, detached commit, primary `●`) comes entirely from
`ListWorktrees`. So the rows can render in ~17ms and the annotations can fill in as each
`git status` completes — a large perceived-speed win with no layout reflow (annotations are
trailing).

## Decision / Scope

Split worktree loading into two stages:

1. **List stage (fast):** emit the worktree rows immediately, with zero-value `Status`
   (no annotations shown yet).
2. **Status stage (async):** fire one command per worktree; as each `git status` returns,
   update that worktree's `Status` and re-render so its annotations appear.

Updates flow back through the Bubble Tea message loop (NOT by goroutines mutating the
model — see gotchas). Concurrency is **unbounded** via `tea.Batch`: the user expects at most
~20 worktrees per project, so ~20 concurrent `git status` processes is acceptable and the
previous semaphore bound is removed.

Decided defaults (do not ask):
- **No placeholder/spinner** on rows while a status is pending — a zero-value `Status`
  simply renders no indicators, and they pop in when ready.
- **Global loading indicator unchanged** — the existing `isLoading`/`loadedCount`
  "Loading worktrees… (n/m projects)" message continues to track the *list* stage only; it
  is not extended to track status fill-in.

## Contract (observable behavior — must hold)

1. On expand, worktree rows (branch/path/primary marker) appear as soon as `ListWorktrees`
   returns, before any `git status` completes.
2. Each worktree's `*` / `↑N` / `↓N` annotations appear once its own `git status` returns,
   and match what the synchronous version produced.
3. If `git status` fails for a worktree, that worktree shows no status annotations (zero
   value) and the rest are unaffected — no error surfaced, no panic. (Matches prior
   continue-on-error behavior.)
4. Worktree **order is unchanged** (the order `ListWorktrees` returned).
5. Re-collapsing and re-expanding a project does **not** refetch the worktree list or
   re-run `git status` (results stay cached in `m.worktrees` / `worktreesLoaded`).
6. Create/delete reload path (`reloadWorktrees`) behaves the same way: rows refresh
   immediately, annotations fill in async.

## Implementation notes (gotchas)

- **Threading is the critical correctness point.** In Bubble Tea the `Model` is owned by the
  update loop; `tea.Cmd` goroutines must report results back as a `tea.Msg` and must NOT
  mutate `Model` state (e.g. the `m.worktrees` map) directly. The current parallel code is
  safe only because it mutates a *local* slice inside one closure before handing it to the
  model. For progressive updates, each status completion MUST return a new message; goroutine
  writes into shared model maps are a data race (`go test -race` will catch it).

- **Use `tea.Batch` for fan-out.** After the list arrives, return
  `tea.Batch(statusCmd_0, statusCmd_1, …)` — Bubble Tea runs them concurrently. This is the
  codebase's first use of `tea.Batch`; `tea` is already imported. If a project has zero
  worktrees, return `nil` rather than an empty batch.

- **New message type** (alongside the others at [model.go:1337-1413](../wt/pkg/tui/model.go#L1337)):
  ```go
  type worktreeStatusLoadedMsg struct {
      projectPath  string
      worktreePath string            // match key (stable, unique)
      status       worktree.GitStatus
  }
  ```

- **New per-worktree command** returns `worktreeStatusLoadedMsg`. On `GetStatus` error,
  return `nil` (Bubble Tea ignores nil messages) — no update for that worktree.

- **New handler** for `worktreeStatusLoadedMsg`: look up `m.worktrees[msg.projectPath]`,
  find the element whose `Path == msg.worktreePath`, set its `Status`, then `buildItems()`.
  Must be a no-op if the project key is absent or no path matches (e.g. worktree deleted
  while a status was in flight).

- **Assigning into a slice held in a map is legal:** `m.worktrees[pp][i].Status = status`
  works because slice elements are addressable (the not-addressable rule applies to structs
  stored directly as map *values*, not to slice elements). Find `i` by matching `Path`.

- **Wiring the two stages:**
  - `loadProjectWorktreesCmd` ([model.go:1250-1305](../wt/pkg/tui/model.go#L1250)): delete the
    `sync.WaitGroup`/semaphore status block (lines ~1279-1297). Return
    `worktreeLoadCompleteMsg` immediately after `ListWorktrees`, with worktrees carrying
    zero-value `Status`.
  - `worktreeLoadCompleteMsg` handler ([model.go:530-547](../wt/pkg/tui/model.go#L530)): after
    storing worktrees and setting `worktreesLoaded[...] = true`, return
    `m, <batch of per-worktree status cmds for msg.worktrees>` instead of `m, nil`.
  - `reloadWorktrees` ([model.go:1485-1507](../wt/pkg/tui/model.go#L1485)): drop its inline
    status loop; return `worktreesLoadedMsg` right after `ListWorktrees`.
  - `worktreesLoadedMsg` handler ([model.go:526-529](../wt/pkg/tui/model.go#L526)): after
    storing worktrees, return the same per-worktree status batch instead of `m, nil`.
  - Eager (non-lazy) startup path `loadAllWorktreesAsync`
    ([model.go:1231-1241](../wt/pkg/tui/model.go#L1231)) uses `loadProjectWorktreesCmd` per
    project and therefore gets progressive status for free — no separate change needed.

- **Remove the now-unused `sync` import** from [model.go](../wt/pkg/tui/model.go#L8) once the
  `sync.WaitGroup` block is gone (verify no other `sync.` usage — currently there is none).

- A small shared helper that builds the `[]tea.Cmd` (one status cmd per worktree) and returns
  `tea.Batch(...)` is recommended so both `worktreeLoadCompleteMsg` and `worktreesLoadedMsg`
  reuse it. Internal naming is at the implementer's discretion.

## Checklist

- [ ] Add `worktreeStatusLoadedMsg` type (projectPath, worktreePath, status).
- [ ] Add a per-worktree status command returning `worktreeStatusLoadedMsg` (nil on error).
- [ ] Add a helper that fans out one status command per worktree as a `tea.Batch` (nil if
      empty).
- [ ] Strip status loading out of `loadProjectWorktreesCmd`; emit `worktreeLoadCompleteMsg`
      with zero-status worktrees right after `ListWorktrees`.
- [ ] Strip status loading out of `reloadWorktrees`; emit `worktreesLoadedMsg` right after
      `ListWorktrees`.
- [ ] Update `worktreeLoadCompleteMsg` handler to return the status batch.
- [ ] Update `worktreesLoadedMsg` handler to return the status batch.
- [ ] Add `worktreeStatusLoadedMsg` handler: match by `Path`, set `Status`, `buildItems()`;
      no-op on missing project/path.
- [ ] Remove the `sync` import (and confirm no remaining `sync.` references).

## Validation

- [ ] `cd wt && go build ./...` succeeds.
- [ ] `cd wt && go vet ./...` reports nothing new (catches the unused `sync` import).
- [ ] `cd wt && go test ./...` — all existing tests pass.
- [ ] `cd wt && go test -race ./pkg/tui/... ./pkg/worktree/...` — passes with the race
      detector (guards the message-based status updates against accidental shared-state
      mutation).
- [ ] Manual smoke: build the binary, run the TUI, expand the Iris project node. Worktree
      rows must appear effectively instantly (well under a second), then `*` / `↑N` / `↓N`
      annotations fill in over the following ~1–3s. Final annotations must match the
      pre-change build for the same repo.
- [ ] Manual: collapse and re-expand the Iris node — no re-fetch / no visible reload delay
      (cached).
- [ ] Manual: create or delete a worktree in a project — the list refreshes immediately and
      annotations fill in async.

## Relevant Files

- [wt/pkg/tui/model.go](../wt/pkg/tui/model.go) — message types (~1337-1413),
  `worktreesLoadedMsg` handler (526-529), `worktreeLoadCompleteMsg` handler (530-547),
  `loadProjectWorktreesCmd` (1250-1305), `loadAllWorktreesAsync` (1231-1241),
  `reloadWorktrees` (1485-1507), import block (3-18). Primary file for all changes.
- [wt/pkg/tui/view.go](../wt/pkg/tui/view.go) — `renderWorktree` (376-426) consumes
  `Status`; reference only (renders zero value as no indicators — no change needed).
- [wt/pkg/worktree/status.go](../wt/pkg/worktree/status.go) — `GetStatus` / `GitStatus`
  (unchanged contract; called by the new per-worktree command).
- [wt/pkg/worktree/worktree.go](../wt/pkg/worktree/worktree.go) — `ListWorktrees` / `Worktree`
  struct (reference only; defines row data and order).
