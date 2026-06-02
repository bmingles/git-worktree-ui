# Parallelize per-worktree `git status` on project expansion

## Problem

When a project node is expanded in the TUI, the worktrees load slowly. Measured on the
Iris repo (11 worktrees, 584 MB `.git`): first expand takes **~7.5s**.

Root cause: [`loadProjectWorktreesCmd`](../wt/pkg/tui/model.go#L1248-L1294) calls
`worktree.GetStatus` once per worktree **sequentially** in the loop at
[model.go:1279-1286](../wt/pkg/tui/model.go#L1279-L1286). Each `git status` on Iris is
~0.47s, so 11 of them serialized ≈ 7.5s.

Measured fix potential: running the same 11 `git status` calls concurrently completes in
**~2.8s** — a ~2.7× speedup with no behavior change. This is a first-expand-only cost
(results are cached in `worktreesLoaded`), so subsequent expands are already instant.

## Decision / Scope

**Parallelize the status loop only.** Run the per-worktree `GetStatus` calls concurrently
instead of sequentially. Concurrency is **bounded** (semaphore) to avoid spawning an
unbounded number of `git` processes for projects with many worktrees.

Explicitly **out of scope** (decided against): the TUI must NOT mutate any repo's
`.git/config` (no `core.untrackedCache`, no `core.fsmonitor`). Per-`git status` speedups
are left to the user to configure manually on their own repos. Do not add config-writing
code.

## Contract (observable behavior — must hold)

1. After expansion, every worktree in the returned slice has its `Status` populated
   exactly as the sequential version would have produced it.
2. **Order is preserved**: the returned `[]worktree.Worktree` slice stays in the same
   order `worktree.ListWorktrees` returned (the UI renders in that order). Achieve this by
   writing each result into its own index `wts[i].Status` — never append from goroutines.
3. **Per-worktree error handling unchanged**: if `GetStatus` fails for a worktree, that
   worktree keeps the zero-value `GitStatus{}` (`HasChanges=false, AheadBy=0, BehindBy=0`)
   and the rest still load. This matches the current `continue`-on-error behavior. The
   overall command must NOT return an error just because one worktree's status failed.
4. The `worktreeLoadCompleteMsg` returned is structurally identical to today (same
   `projectPath`, populated `worktrees`, `err` only set for the `ListWorktrees` failure or
   project-not-found cases that already return early).

## Implementation notes (gotchas)

- **Writing distinct slice indices from multiple goroutines is race-free in Go** (separate
  memory locations, slice is not resized). `wts[i].Status = status` inside goroutine `i` is
  safe. Do NOT share a single accumulator or append concurrently.
- Use `sync.WaitGroup` to wait for all goroutines. Bound concurrency with a buffered
  channel used as a semaphore — cap at a constant (e.g. `maxConcurrentStatus = 16`). With
  11 worktrees all run at once; the cap only matters for projects with many worktrees.
- This is the codebase's first use of `sync` — add the `sync` import to
  [model.go](../wt/pkg/tui/model.go). No third-party `errgroup` dependency; standard
  library only (`go 1.25.6`).
- Capture the loop variable correctly per goroutine (`i` by value). Go 1.25 has per-iteration
  loop variables, but pass `i` explicitly or shadow it to be unambiguous.
- The work runs inside a `tea.Cmd` closure already off the UI goroutine — adding goroutines
  inside it is fine; just ensure they all complete (`wg.Wait()`) before building the
  `worktreeLoadCompleteMsg`.
- Internal naming (helper function vs. inline, semaphore var names) is at the implementing
  agent's discretion. A clean option is a `worktree.LoadStatuses([]Worktree)` helper in the
  `worktree` package, but inlining in `loadProjectWorktreesCmd` is equally acceptable.

## Checklist

- [ ] Replace the sequential `for i := range wts { GetStatus... }` loop in
      `loadProjectWorktreesCmd` with a bounded-concurrency version using `sync.WaitGroup`
      + a semaphore channel.
- [ ] Each goroutine writes only `wts[i].Status` (preserve slice order); on `GetStatus`
      error, leave the zero-value status and continue (no panic, no early return).
- [ ] Add the `sync` import to the file.
- [ ] Define a concurrency-cap constant (`maxConcurrentStatus = 16` or similar).
- [ ] Confirm no other call site relied on the sequential timing/ordering.

## Validation

- [ ] `cd wt && go build ./...` succeeds.
- [ ] `cd wt && go vet ./...` reports nothing new.
- [ ] `cd wt && go test ./...` — all existing tests pass.
- [ ] `cd wt && go test -race ./pkg/tui/... ./pkg/worktree/...` — passes with the race
      detector enabled (guards the concurrent slice writes).
- [ ] Manual smoke: build the binary, run the TUI, expand the Iris project node. First
      expand should complete in roughly 2–3s instead of ~7.5s, and the worktree list /
      dirty-state indicators / ahead-behind counts must match what the sequential version
      showed (compare against current `main` build for the same repo).

## Relevant Files

- [wt/pkg/tui/model.go](../wt/pkg/tui/model.go) — `loadProjectWorktreesCmd`
  (lines ~1248-1294); the status loop at ~1279-1286 is what changes. Add `sync` import.
- [wt/pkg/worktree/status.go](../wt/pkg/worktree/status.go) — `GetStatus` (unchanged
  contract; referenced by the parallel loop). Only touched if a batch helper is added here.
- [wt/pkg/worktree/worktree.go](../wt/pkg/worktree/worktree.go) — `ListWorktrees` /
  `Worktree` struct (reference only; defines the slice order that must be preserved).
- [wt/pkg/worktree/status_test.go](../wt/pkg/worktree/status_test.go) — existing parse
  tests (must still pass; extend only if adding a batch helper worth unit-testing).
