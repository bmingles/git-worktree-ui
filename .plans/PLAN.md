# Plan Status

## Status

### Pending

*(none)*

### Completed

- [async-worktree-status](async-worktree-status.md) — load worktree `git status` asynchronously so rows render instantly and `*`/`↑`/`↓` annotations fill in progressively.
- [parallelize-worktree-status](archived/parallelize-worktree-status.md) — ran per-worktree
  `git status` concurrently (bounded) on expansion (first-expand ~7.5s → ~2.8s on Iris).

## Development Phases

| Plan | Description | Status |
|------|-------------|--------|
| [parallelize-worktree-status](archived/parallelize-worktree-status.md) | Run per-worktree `git status` calls concurrently (bounded) instead of sequentially when expanding a project node | complete |
| [async-worktree-status](async-worktree-status.md) | Render worktree rows immediately from `ListWorktrees`, fetch `git status` asynchronously, fill in annotations as each returns | complete |
