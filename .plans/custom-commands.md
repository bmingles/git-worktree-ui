# Custom commands (config-defined shell shortcuts, scoped to node type)

## Problem

The TUI exposes a fixed set of built-in actions (add/delete worktree, open in VS Code,
create workspace/devcontainer, assign category/tags). Users want to wire their own shell
commands to keys — e.g. `lazygit` on a worktree, `git fetch --all` on a project — and have
them appear in the contextual help footer when an applicable node is selected.

Today there is no config surface for this, no execution path for arbitrary shell commands,
and the help footer ([view.go renderHelp](../wt/pkg/tui/view.go#L428)) is hard-coded per
`ItemType`. Built-in git/vscode commands run via `exec.Command(...).CombinedOutput()` with
`cmd.Dir` set (see [worktree/operations.go](../wt/pkg/worktree/operations.go),
[vscode/vscode.go](../wt/pkg/vscode/vscode.go)) — fire-and-forget, no tty handoff.

## Decision / Scope

Add a top-level `commands:` list to the config. Each entry maps a **key** to a **shell
command**, tagged with a **scope** that controls which node type(s) the command is offered on.
In-scope commands are appended to the contextual help footer and dispatched from the normal
key handler. The selected node determines the command's working directory.

Decided (from product Q&A — do not re-ask):

1. **Execution model — suspend TUI and run in the terminal** via `tea.ExecProcess`. The TUI
   releases the terminal, the command gets full stdin/stdout/tty (so interactive tools like
   `lazygit`/`npm install` work as well as quick ones like `git fetch`), and the TUI redraws
   on exit. This is the codebase's first use of `tea.ExecProcess`; only `os/exec` +
   `tea.ExecProcess` are needed (no output-capture modal in v1).
2. **Schema — single top-level `commands:` list** with a `scope` field (not per-project).
   Applies across all projects.
3. **Key conflicts — built-in keys are reserved; colliding custom keys are rejected at config
   load** with a clear error. No shadowing of built-in actions.

In scope:
- `Command` struct + `Config.Commands` field + YAML round-trip.
- Config validation (reserved keys, valid scope enum, non-empty key/command, no duplicate
  `(key, scope)` among custom commands).
- Contextual help footer shows in-scope custom commands.
- Key dispatch runs the matching in-scope command for the selected node via `tea.ExecProcess`.
- Working-directory resolution (worktree path / project path, `+subfolder` when configured).
- Context env vars passed to the command.
- **Command arguments**: a command declares named args with command-level **default** values;
  each project may **override** any of them. Resolved values are injected as `WT_ARG_<name>`
  env vars, so the command line references them (e.g. `--template "$WT_ARG_template"`).
- After a command exits, reload the affected project's worktrees so status annotations
  refresh (a command like `git fetch`/`commit` changes ahead/behind/dirty state).

Decided for command args (locked):

4. **Args are passed as `WT_ARG_<name>` environment variables, not string-substituted into the
   command line.** Rationale: consistent with the existing `WT_*` context vars, `sh -c` expands
   `$WT_ARG_x` natively, and it avoids both a templating engine and the shell-injection/quoting
   hazard of splicing arbitrary config values into a command string. The user quotes the var in
   their command (`"$WT_ARG_template"`). (Alternative considered: `${name}` substitution in the
   `command` string — rejected for the injection surface.)

Out of scope (note in PR, do not build): output-capture modal mode, per-project command
lists, leader-key namespacing, multi-key chords beyond what `tea.KeyMsg.String()` already
yields (e.g. `ctrl+g` is fine; sequences are not), `wt config command add/remove` CLI
subcommands (config is hand-edited via `[e]` for v1).

## Config schema

New struct in [config/config.go](../wt/pkg/config/config.go) alongside `Project`:

```go
// Command is a user-defined shell command bound to a key and offered on nodes of a
// given scope. cwd is derived from the selected node (see "Scope → working directory").
type Command struct {
	Key     string            `yaml:"key"`            // e.g. "g", "F", "ctrl+g" — must equal tea.KeyMsg.String()
	Label   string            `yaml:"label"`          // shown in the help footer, e.g. "lazygit"
	Scope   string            `yaml:"scope"`          // "worktree" | "project" | "any"
	Command string            `yaml:"command"`        // shell line, run via `sh -c`
	Args    map[string]string `yaml:"args,omitempty"` // NEW: arg name -> default value; injected as $WT_ARG_<name>
}
```

`Project` gains a per-project override map:

```go
type Project struct {
	Name        string            `yaml:"name"`
	Path        string            `yaml:"path"`
	Tags        []string          `yaml:"tags,omitempty"`
	Category    string            `yaml:"category,omitempty"`
	Color       string            `yaml:"color,omitempty"`
	SubFolder   string            `yaml:"subfolder,omitempty"`
	CommandArgs map[string]string `yaml:"command_args,omitempty"` // NEW: arg name -> value, overrides Command.Args defaults
}
```

`Config` gains:

```go
type Config struct {
	Projects   []Project `yaml:"projects"`
	Categories []string  `yaml:"categories,omitempty"`
	Commands   []Command `yaml:"commands,omitempty"` // NEW
}
```

Example (append to [example-config.yaml](../wt/example-config.yaml)):

```yaml
commands:
  - key: g
    label: lazygit
    scope: worktree            # runs in the selected worktree's checkout (+subfolder)
    command: lazygit
  - key: F
    label: fetch
    scope: project             # runs in the project path (+subfolder)
    command: git fetch --all --prune
  - key: P
    label: push branch
    scope: worktree
    command: git push -u origin "$WT_BRANCH"
  - key: s
    label: scaffold
    scope: project
    command: project-cli --template "$WT_ARG_template"   # references the resolved arg
    args:
      template: default-template     # command-level fallback used when a project omits it

projects:
  - name: api-service
    path: /Users/username/code/my-api
    command_args:
      template: api-template         # overrides the 'template' default for this project only
  # dashboard has no command_args -> 'project-cli --template default-template'
```

### Command arguments (project-overridable, with defaults)

- A command's `args:` map declares the named arguments it consumes and their **default**
  (fallback) values.
- A project's `command_args:` map supplies **overrides** by the same names. The namespace is
  flat and shared across all commands — a project's `template` value applies to every command
  that declares a `template` arg.
- **Resolution per dispatch:** for each name in the union of `command.Args` keys and the
  selected project's `command_args` keys, value = `project.CommandArgs[name]` if present, else
  `command.Args[name]` (the default), else empty string. Each resolved pair is exported to the
  command's environment as `WT_ARG_<name>` (the name is preserved verbatim, e.g. `template` →
  `$WT_ARG_template`).
- A project override for a name no command declares is harmless (exported, unused). A command
  arg with no project override falls back to its default; a default of `""` (or missing) yields
  an empty env value — the command is responsible for handling that.

### Scope → node type → working directory

| `scope`     | Offered on node types        | Working directory (`cmd.Dir`)                              |
|-------------|------------------------------|------------------------------------------------------------|
| `worktree`  | Worktree only                | `GetTargetPath(item.Worktree.Path, project.SubFolder)`     |
| `project`   | Project only                 | `GetTargetPath(item.ProjectPath, project.SubFolder)`       |
| `any`       | Project **and** Worktree     | worktree → worktree path; project → project path (each `+subfolder`) |

> **Interpretation note (worktree cwd):** the request said worktree-scoped commands run "in
> the project path or in the subfolder if configured." For a *worktree* node the meaningful
> directory is the **selected worktree's checkout** (`item.Worktree.Path`), not the primary
> repo path — otherwise the command would ignore which worktree is selected. This mirrors how
> [openInVSCode](../wt/pkg/tui/model.go#L1413) already resolves a path: worktree items use
> `item.Worktree.Path`, project items use `item.ProjectPath`, then `workspace.GetTargetPath`
> applies the project's `SubFolder`. We reuse that exact resolution. Category nodes are not a
> command scope in v1 (they are not selectable and have no path).

### Context environment variables

The spawned command inherits the parent environment plus:

| Var               | Value                                                | Available when        |
|-------------------|------------------------------------------------------|-----------------------|
| `WT_PROJECT_NAME` | `item.ProjectName`                                   | project + worktree    |
| `WT_PROJECT_PATH` | `item.ProjectPath`                                   | project + worktree    |
| `WT_CWD`          | resolved working directory (the `cmd.Dir` above)     | always                |
| `WT_WORKTREE_PATH`| `item.Worktree.Path`                                 | worktree only         |
| `WT_BRANCH`       | `item.Worktree.Branch`                               | worktree only         |
| `WT_SCOPE`        | the matched command's scope string                   | always                |
| `WT_ARG_<name>`   | resolved arg value (project override → command default → `""`) | one per declared/overridden arg |

## Contract (observable behavior — must hold)

1. A `commands:` entry whose `scope` matches the selected node's type causes its `[key] label`
   to appear in the contextual help footer for that node (and only that node type).
2. Pressing that key while an in-scope node is selected suspends the TUI, runs
   `sh -c "<command>"` with `cmd.Dir` = the resolved working directory and the context env
   vars set, then returns to the TUI on exit.
3. Built-in keys (`q ctrl+c esc / up down k j enter o space right left l h n a d c t v i e r`)
   are reserved. A config whose `commands:` reuses any of them, omits `key`/`label`/`command`,
   uses an invalid `scope`, or defines two entries with the same `(key, scope)` pair fails to
   load with a clear, actionable error naming the offending key/scope.
4. A custom key is **never** dispatched when its scope does not match the selected node (e.g. a
   `worktree`-scoped key pressed on a project node is a no-op).
5. Custom keys are inert while any input/confirm/search mode is active (they only fire in
   normal navigation, like every other built-in action key).
6. If the command exits non-zero (or fails to start), the error and exit status surface in the
   existing status/error line; the TUI still resumes cleanly.
7. After the command exits, the selected node's project worktrees reload so `*` / `↑N` / `↓N`
   annotations reflect any git state the command changed. (Reuses the existing async reload
   path; no blocking.)
8. Each declared/overridden arg is present in the command's environment as `WT_ARG_<name>`,
   with the project's `command_args` value when set, otherwise the command's `args` default,
   otherwise empty string. A project without a `command_args` entry for a name gets the
   default; the value is identical regardless of which in-scope node of that project is
   selected.
9. Existing configs without a `commands:` key load and behave exactly as before (field is
   `omitempty`, nil slice → no custom commands).

## Implementation notes (gotchas)

- **`tea.ExecProcess` is the dispatch primitive.** Build `exec.Command("sh", "-c", cmd.Command)`,
  set `.Dir` and `.Env`, and return
  `tea.ExecProcess(c, func(err error) tea.Msg { return customCommandFinishedMsg{...} })`
  from `handleKeyPress`. Bubble Tea handles the terminal release/restore. Do **not** call
  `CombinedOutput()` — that would capture the tty and break interactivity.

- **Build `.Env` by appending to `os.Environ()`**, not replacing it, so `PATH` etc. survive.
  Only set `WT_WORKTREE_PATH`/`WT_BRANCH` for worktree nodes. Append the resolved `WT_ARG_*`
  pairs last.

- **Arg resolution helper.** Add `func ResolveArgs(cmd Command, p *Project) map[string]string`
  (or a method) in `config`: start from `cmd.Args` (defaults), then overlay any matching keys
  from `p.CommandArgs`. Keys present only in `p.CommandArgs` are still included (harmless). The
  dispatcher turns the result into `WT_ARG_<name>=<value>` env entries. Keep the helper in
  `config` so it is unit-testable without the TUI. Look up the `*Project` by `item.ProjectPath`
  (same lookup already used for `SubFolder`).

- **Reserved-key set is the single source of truth.** Define an exported
  `var ReservedKeys = map[string]bool{...}` in the `config` package (it is just string data, and
  `config` must reach it from `ValidateCommands`) with a comment pointing at
  [model.go handleKeyPress](../wt/pkg/tui/model.go#L675), since `config` must not import `tui`
  (layering / cycle risk). Add a one-line note in `handleKeyPress` that adding a new built-in
  key requires updating `config.ReservedKeys`. The list to seed it with, verbatim from the
  current `handleKeyPress` cases: `q`, `ctrl+c`, `esc`, `/`, `up`, `down`, `k`, `j`, `enter`,
  `o`, `" "` (space), `right`, `left`, `l`, `h`, `n`, `a`, `d`, `c`, `t`, `v`, `i`, `e`, `r`.

- **Validation entry point.** Add `func (c *Config) ValidateCommands() error` in `config` and
  call it at the end of `LoadConfig` ([config.go:95](../wt/pkg/config/config.go#L95)) so a bad
  config surfaces immediately at TUI startup (and anywhere else config loads). Errors should
  name the key + scope, e.g. `custom command: key "d" is reserved by a built-in action`.
  Required non-empty fields: `key`, `label` (Decision 1), `command`. Validate `scope` ∈
  {`worktree`,`project`,`any`}.
  Also validate **arg names**: each key in `command.Args` (and each `project.command_args` key)
  must match `^[A-Za-z_][A-Za-z0-9_]*$` so it forms a legal `WT_ARG_<name>` shell variable;
  reject with a clear error naming the bad arg.

- **Help footer wiring (concrete).** `renderHelp` ([view.go:429](../wt/pkg/tui/view.go#L429))
  builds a `var rows []string` (each element is one footer row joined by `"\n\n"`), fills it in a
  `switch item.Type`, then appends `globalCommands` and returns
  `helpStyle.Render(strings.Join(rows, "\n\n"))`. Inject custom commands **after the `switch`
  and before `globalCommands` is built** (so they appear above the global row for every item
  type), e.g.:
  ```go
  if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
      if seg := m.customCommandHelp(m.items[m.selectedIndex].Type); seg != "" {
          rows = append(rows, seg)
      }
  }
  ```
  Add the helper `func (m Model) customCommandHelp(t ItemType) string` (in view.go): for each
  `m.commands` where `commandAppliesTo(c, t)`, build `"[" + c.Key + "] " + c.Label` and join with
  `" • "`; return `""` if none. (`Label` is guaranteed non-empty by validation — Decision 1.)
  Reuse the existing `•` separator + `helpStyle`.

- **`commands` field + constructor signature (don't break the build).** Add a
  `commands []config.Command` field to `Model`. The constructor is
  `func NewModel(projects []config.Project, categories []string) Model`
  ([model.go:87](../wt/pkg/tui/model.go#L87)) — change it to
  `NewModel(projects []config.Project, categories []string, commands []config.Command)` and
  set `commands: commands` in the struct literal ([model.go:126](../wt/pkg/tui/model.go#L126)).
  **All call sites must be updated or compilation fails:**
  - [cmd/root.go:61](../wt/cmd/root.go#L61): `tui.NewModel(cfg.Projects, cfg.Categories, cfg.Commands)`.
  - [model_test.go:39](../wt/pkg/tui/model_test.go#L39), `:101`, `:142`: pass a third arg
    (`nil` where the test doesn't exercise commands).

- **Refresh `m.commands` at the config-reload sites.** Three handlers in `Update` re-`LoadConfig`
  and re-set `m.projects`/`m.categories`: `projectAddedMsg` (~[model.go:600](../wt/pkg/tui/model.go#L600)),
  `categoryAssignedMsg` (~625), `tagsAssignedMsg` (~646). Add `m.commands = cfg.Commands` in each
  so an edited `commands:` list is picked up on the same refreshes that already pick up projects.
  (There is no separate "reload config" key today — `[e]` opens the file, `[r]` reloads only
  worktrees — so this matches existing project-refresh behavior; do not add a new reload path.)

- **`tea.WithAltScreen` is fine.** The program runs with `tea.WithAltScreen()`
  ([root.go:62](../wt/cmd/root.go#L62)); `tea.ExecProcess` correctly exits the alt-screen, runs
  the child on the real terminal, and restores the alt-screen + redraws on return. No extra
  handling needed.

- **Scope → ItemType match helper.** A small predicate
  `commandAppliesTo(cmd config.Command, t ItemType) bool`: `worktree` ↔ `ItemTypeWorktree`,
  `project` ↔ `ItemTypeProject`, `any` ↔ either. **Must live in the `tui` package** (it
  references `ItemType`, a `tui` type — putting it in `config` would create a `config → tui`
  import cycle). Used by both the footer renderer and the key dispatcher so they cannot drift.

- **Key dispatch placement (concrete).** `handleKeyPress` is only reached in normal navigation
  (input modes are handled earlier in `Update`, lines ~206–509), so custom keys are
  automatically inert during input — no extra guard needed. Custom keys won't match any built-in
  `case`. Add the lookup at the **end** of `handleKeyPress`, replacing the final `return m, nil`
  ([model.go:932](../wt/pkg/tui/model.go#L932)):
  ```go
  // custom command dispatch (built-in keys are reserved, so no overlap)
  if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
      item := m.items[m.selectedIndex]
      for _, c := range m.commands {
          if c.Key == msg.String() && commandAppliesTo(c, item.Type) {
              return m, m.runCustomCommand(c, item)
          }
      }
  }
  return m, nil
  ```
  Add `func (m Model) runCustomCommand(c config.Command, item Item) tea.Cmd` that resolves cwd +
  env (above), builds `exec.Command("sh", "-c", c.Command)`, sets `.Dir`/`.Env`, and returns
  `tea.ExecProcess(cmd, func(err error) tea.Msg { return customCommandFinishedMsg{err: err,
  label: c.Label, projectPath: item.ProjectPath} })`.

- **cwd resolution (use the existing helper).** A model helper already exists:
  `func (m Model) getProjectSubFolder(projectPath string) string`
  ([view.go](../wt/pkg/tui/view.go), just below `renderHelp`). Do **not** hand-roll the project
  loop. Resolve cwd as:
  ```go
  base := item.ProjectPath
  if item.Type == ItemTypeWorktree && item.Worktree != nil {
      base = item.Worktree.Path
  }
  cwd := workspace.GetTargetPath(base, m.getProjectSubFolder(item.ProjectPath))
  ```
  (`openInVSCode` at [model.go:1413](../wt/pkg/tui/model.go#L1413) currently inlines the same
  loop; matching it via `getProjectSubFolder` is fine — an optional follow-up could switch
  `openInVSCode` to the helper too, but that's not required here.)

- **Post-run refresh (no guards needed).** The `customCommandFinishedMsg` handler should set
  `m.err` on failure, then return `m.reloadWorktrees(msg.projectPath)` as its `tea.Cmd`.
  `reloadWorktrees(projectPath) tea.Cmd` ([model.go](../wt/pkg/tui/model.go), `func (m Model)
  reloadWorktrees`) re-runs `ListWorktrees` and emits `worktreesLoadedMsg`, whose handler stores
  the result into `m.worktrees` unconditionally and (per the async-status work) fans out
  `git status`. It is safe and cheap to call for any project node regardless of expansion state,
  so **no collapsed/loaded guard is required** — just capture `projectPath` in the message and
  call it.

- **New message type** (alongside the others at
  [model.go:1328-1413](../wt/pkg/tui/model.go#L1328)):
  ```go
  type customCommandFinishedMsg struct {
      err         error
      label       string
      projectPath string // to refresh after run
  }
  ```

- **`os/exec` import** must be added to [model.go imports](../wt/pkg/tui/model.go#L3) (currently
  not imported there).

- **Don't break the help footer width logic.** The footer already builds multi-segment rows
  joined with `•`; appending custom-command segments should follow the same style and respect
  the existing `helpStyle`. If many commands are configured the row may wrap — acceptable for
  v1 (note it, don't solve it).

## Checklist

- [ ] Add `Command` struct (incl. `Args`) and `Config.Commands` field (`omitempty`) in
      `config.go`; add `Project.CommandArgs` (`omitempty`).
- [ ] Add `config.ReservedKeys` set (seeded from the built-in key list above) with a sync note.
- [ ] Add `(*Config).ValidateCommands()` (reserved key, scope enum, non-empty
      key/label/command, duplicate `(key,scope)`, arg-name pattern), and call it from `LoadConfig`.
- [ ] Add `config.ResolveArgs(cmd, project)` (defaults overlaid by project overrides).
- [ ] Add `commandAppliesTo(cmd config.Command, t ItemType)` predicate in the **`tui`** package
      (not `config` — avoids an import cycle).
- [ ] Add `commands []config.Command` to `Model`; extend `NewModel` signature + struct literal;
      update all 4 call sites (`cmd/root.go:61` passes `cfg.Commands`; 3 `model_test.go` sites
      pass `nil`).
- [ ] Set `m.commands = cfg.Commands` in the three config-reload handlers (`projectAddedMsg`,
      `categoryAssignedMsg`, `tagsAssignedMsg`).
- [ ] Append in-scope custom commands to `renderHelp` for project + worktree selections.
- [ ] Add custom-command lookup + `tea.ExecProcess` dispatch at the end of `handleKeyPress`
      (resolve cwd via `workspace.GetTargetPath`, build `.Env` from `os.Environ()` + WT_* vars
      + `WT_ARG_*` from `ResolveArgs`, run `sh -c`).
- [ ] Add `customCommandFinishedMsg` type + handler (surface error, async-reload the project).
- [ ] Add `os/exec` import to `model.go`.
- [ ] Document the feature + example block in `example-config.yaml` and `README.md`.

## Validation

- [ ] `cd wt && go build ./...` succeeds.
- [ ] `cd wt && go vet ./...` reports nothing new.
- [ ] `cd wt && go test ./...` — existing tests pass.
- [ ] New unit tests in `config_test.go`: `ValidateCommands` rejects a reserved key, an invalid
      scope, an empty key, an empty label, an empty command, duplicate `(key,scope)`, and an
      illegal arg name; accepts a valid set; a config with no `commands:` loads with a nil slice.
- [ ] New unit test for `ResolveArgs`: project override wins; command default used when project
      omits the key; project-only key included; empty default → empty value.
- [ ] New unit test for `commandAppliesTo` (in `model_test.go`, `tui` package) covering
      worktree/project/any × each ItemType.
- [ ] `cd wt && go test -race ./pkg/tui/...` passes (dispatch returns a `tea.Cmd`; no shared
      mutation).
- [ ] Manual smoke (build binary, run TUI):
  - [ ] Config with `scope: worktree` command (`lazygit`): footer shows it only on worktree
        rows; pressing the key suspends the TUI, launches the tool in that worktree's dir,
        and the TUI redraws on exit.
  - [ ] `scope: project` command (`git fetch --all`): offered only on project rows; runs in the
        project dir; on exit the worktree ahead/behind annotations refresh.
  - [ ] `scope: any` command appears on both; cwd differs correctly per node.
  - [ ] A subfolder-configured project runs the command in `<path>/<subfolder>`.
  - [ ] `echo "$WT_BRANCH $WT_PROJECT_NAME"` confirms env vars are populated.
  - [ ] Arg fallback: a `scaffold` command with `args: {template: default-template}` run on a
        project with no `command_args` echoes `default-template`; run on a project with
        `command_args: {template: api-template}` echoes `api-template`.
  - [ ] A command exiting non-zero surfaces the error in the status line and the TUI recovers.
  - [ ] A config reusing key `d` fails to load with a clear "reserved key" error.

## Decisions (resolved — no open questions for the implementer)

These were evaluated and locked; implement them as stated, do not re-decide:

1. **`label` is required.** `ValidateCommands` rejects an empty/missing `label` (same as
   `key`/`command`). The footer always renders `[key] label`; there is **no** fallback logic.
2. **No new config-reload path.** `[r]` continues to reload only worktrees; `[e]` only opens the
   file. An edited `commands:` list is picked up on the same events that already re-read config
   (`projectAdded`/`categoryAssigned`/`tagsAssigned`) and on restart — identical to how
   `projects:` behaves today. Do not add a config-reload to `[r]`.
3. **No footer overflow handling in v1.** A long row of in-scope commands may wrap; that is
   accepted. Do not add truncation/paging. (Just confirm it's not visually broken in the smoke
   test; if it is, that's a separate follow-up, not part of this work.)
4. **Unix-only via `sh -c`.** Commands run through `sh -c`, consistent with the tool's existing
   reliance on a Unix environment (`code`, git worktrees). No Windows / non-POSIX-shell handling.

## Relevant Files

- [wt/pkg/config/config.go](../wt/pkg/config/config.go) — `Command` struct (incl. `Args`),
  `Config.Commands`, `Project.CommandArgs`, `ReservedKeys`, `ValidateCommands`, `ResolveArgs`,
  hook into `LoadConfig` (95-122). Primary schema file.
- [wt/pkg/config/config_test.go](../wt/pkg/config/config_test.go) — `ValidateCommands` +
  `ResolveArgs` tests (`commandAppliesTo` test goes in `model_test.go`, since the predicate is
  in `tui`).
- [wt/pkg/tui/model.go](../wt/pkg/tui/model.go) — `Model` gains `commands`; `NewModel` (87) +
  struct literal (126); config-reload handlers (~600/625/646) set `m.commands`; `handleKeyPress`
  (675-933) dispatch tail; new `customCommandFinishedMsg` (~1328-1413) + handler in `Update`;
  cwd/env resolution mirroring `openInVSCode` (1413-1452); `os/exec` import (3-16). Primary
  behavior file.
- [wt/cmd/root.go](../wt/cmd/root.go) — `NewModel` call site (61) passes `cfg.Commands`.
- [wt/pkg/tui/model_test.go](../wt/pkg/tui/model_test.go) — 3 `NewModel` call sites (39/101/142)
  need the new third arg.
- [wt/pkg/tui/view.go](../wt/pkg/tui/view.go) — `renderHelp` (428-523) appends in-scope custom
  commands per `ItemType`.
- [wt/pkg/workspace/workspace.go](../wt/pkg/workspace/workspace.go) — `GetTargetPath` (17-22),
  reused for cwd resolution (reference only).
- [wt/example-config.yaml](../wt/example-config.yaml) / [wt/README.md](../wt/README.md) — docs.
