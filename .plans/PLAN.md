# Plan Status

## Status

### Pending

- [custom-commands](custom-commands.md) — config-defined `commands:` mapping a key to a shell command, scoped to node type (worktree/project/any); in-scope commands show in the help footer and run via `tea.ExecProcess` in the node's directory (+subfolder), with reserved built-in keys rejected at load. Commands declare named `args` with defaults, project-overridable via `command_args`, injected as `WT_ARG_*` env vars.

### Completed

- [async-worktree-status](async-worktree-status.md) — load worktree `git status` asynchronously so rows render instantly and `*`/`↑`/`↓` annotations fill in progressively.
- [parallelize-worktree-status](archived/parallelize-worktree-status.md) — ran per-worktree
  `git status` concurrently (bounded) on expansion (first-expand ~7.5s → ~2.8s on Iris).

## Development Phases

| Plan | Description | Status |
|------|-------------|--------|
| [parallelize-worktree-status](archived/parallelize-worktree-status.md) | Run per-worktree `git status` calls concurrently (bounded) instead of sequentially when expanding a project node | complete |
| [async-worktree-status](async-worktree-status.md) | Render worktree rows immediately from `ListWorktrees`, fetch `git status` asynchronously, fill in annotations as each returns | complete |
| [custom-commands](custom-commands.md) | Config `commands:` list (key → shell command, scoped to node type) shown contextually in the help footer and run via `tea.ExecProcess` in the selected node's directory | pending |
