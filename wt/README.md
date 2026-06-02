# wt - Git Worktree Manager

A command-line tool for managing Git worktrees with an interactive TUI interface.

## Features

- **Interactive TUI**: Navigate through your projects and worktrees with an intuitive interface
- **Quick VS Code Integration**: Open any worktree in VS Code with a single keystroke
- **Worktree Management**: Create and delete worktrees directly from the TUI
- **Workspace File Generation**: Add color-coded `.local.code-workspace` files to projects and worktrees
- **Git Status Display**: View branch status, changes, and commit position at a glance
- **Configuration Management**: Simple YAML-based configuration for multiple projects
- **Keyboard-Driven**: Efficient keyboard navigation for fast workflow

## Installation

To install `wt`, you need Go 1.25 or later. Install using:

```bash
go install github.com/bmingles/wt@latest
```

The binary will be installed to your `$GOPATH/bin` directory (typically `~/go/bin`). Make sure this directory is in your `PATH`.

### Building from Source

```bash
git clone https://github.com/bmingles/wt
cd wt
go build
```

The `wt` binary will be created in the current directory. Move it to a directory in your `PATH` or run `go install` to install it to `$GOPATH/bin`.

## Quick Start

1. Initialize the configuration file:
   ```bash
   wt config init
   ```

2. Add a project to track:
   ```bash
   wt config add my-project /path/to/project
   ```

3. Launch the TUI:
   ```bash
   wt
   ```

## CLI Commands

### `wt`

Launch the interactive TUI interface. This is the default command when no subcommand is provided.

```bash
wt [--config <path>]
```

**Flags:**
- `--config`: Path to custom config file (default: `~/.config/wt/config.yaml`)

### `wt config`

Manage wt configuration including projects and settings.

#### `wt config init`

Initialize a default config file at `~/.config/wt/config.yaml`.

```bash
wt config init [--config <path>]
```

#### `wt config add <name> <path>`

Add a new project to the configuration.

```bash
wt config add <name> <path> [--config <path>]
```

**Arguments:**
- `<name>`: Unique name for the project
- `<path>`: Absolute or relative path to the project directory

**Example:**
```bash
wt config add dashboard ~/code/vscode-dashboard
wt config add api ../api-service
```

#### `wt config list`

Display all configured projects.

```bash
wt config list [--config <path>]
```

## TUI Keyboard Shortcuts

### Navigation

- **↑ / k**: Move selection up
- **↓ / j**: Move selection down

### Actions

- **Enter / o**: Open selected worktree in VS Code
- **v**: Create a `.local.code-workspace` file for the selected project or worktree
  - Generates workspace file with unique color customization
  - Colors are stored in project config and can be manually customized
  - New projects get auto-generated colors based on MD5 hash of the project path
  - Foreground colors automatically adjust for optimal contrast
  - Worktrees use the same color as their primary project
  - Hidden if workspace file already exists
- **c**: Create new worktree for the selected project
  - Enter branch name and press Enter to create
  - Press Esc to cancel
- **d**: Delete selected worktree
  - Confirm with 'y' or cancel with 'n' / Esc
  - Primary worktrees cannot be deleted
- **q / Ctrl+C**: Quit the application

### Confirmation Dialogs

When in confirmation mode (after pressing 'd'):
- **y / Y**: Confirm action
- **n / N / Esc**: Cancel action

### Input Mode

When creating a worktree (after pressing 'c'):
- **Enter**: Create worktree with entered branch name
- **Esc / Ctrl+C**: Cancel and return to navigation

## Configuration

The configuration file is stored at `~/.config/wt/config.yaml` by default. You can specify a custom location using the `--config` flag.

### Configuration Format

```yaml
projects:
  - name: dashboard
    path: /Users/username/code/vscode-dashboard
    color: "3498db"  # Optional: 6-char hex color (without #)
  - name: api
    path: /Users/username/code/api-service
    color: "e74c3c"
  - name: frontend
    path: /Users/username/code/frontend-app
    # No color specified - will auto-generate from path hash
```

Each project has:
- **name**: A unique identifier for the project
- **path**: Absolute path to the Git repository
- **color** (optional): 6-character hex color code (e.g., "3498db") for workspace/devcontainer theming
  - Automatically generated when projects are created via the TUI
  - Can be manually edited in the config file
  - If omitted or empty, falls back to hash-based color generation
  - Applies to both the project and all its worktrees
- **subfolder** (optional): A subfolder path relative to the checkout root, for monorepo projects

### Subfolder Configuration

For monorepo projects, you can specify a subfolder where workspace files, devcontainer files, and VS Code should open. When set, all TUI operations (workspace file creation, devcontainer creation, opening in VSCode) will target the subfolder relative to the checkout root.

```yaml
projects:
  - name: my-monorepo
    path: /Users/username/code/my-monorepo
    color: "27ae60"
    subfolder: packages/frontend
  - name: backend
    path: /Users/username/code/my-monorepo
    color: "8e44ad"
    subfolder: packages/backend
```

This is useful when:
- Your repository contains multiple independent packages or apps
- You want VS Code to open to a specific package's directory
- Each sub-project has its own devcontainer or workspace configuration

See [example-config.yaml](example-config.yaml) for a complete example.

### Custom Commands

You can bind arbitrary shell commands to keys via the `commands:` list in your config. Each custom command is shown in the contextual help footer when an applicable node is selected, and the TUI suspends to let the command run interactively (e.g. `lazygit`, `npm install`) before returning.

```yaml
commands:
  - key: g
    label: lazygit
    scope: worktree          # "worktree" | "project" | "any"
    command: lazygit

  - key: F
    label: fetch
    scope: project
    command: git fetch --all --prune

  - key: P
    label: push branch
    scope: worktree
    command: git push -u origin "$WT_BRANCH"
```

**Scope** controls which node types show and activate the command:
- `worktree` — offered only when a worktree row is selected; `cwd` is the worktree checkout path
- `project` — offered only when a project row is selected; `cwd` is the project path
- `any` — offered on both; `cwd` resolves per node type

**Environment variables** available to every command:

| Variable | Value |
|---|---|
| `WT_PROJECT_NAME` | project name |
| `WT_PROJECT_PATH` | project root path |
| `WT_CWD` | resolved working directory |
| `WT_SCOPE` | the command's scope string |
| `WT_WORKTREE_PATH` | worktree path (worktree nodes only) |
| `WT_BRANCH` | branch name (worktree nodes only) |
| `WT_ARG_<name>` | per-arg values (see below) |

**Command arguments** let you declare named parameters with defaults that individual projects can override:

```yaml
commands:
  - key: s
    label: scaffold
    scope: project
    command: project-cli --template "$WT_ARG_template"
    args:
      template: default-template   # fallback when a project doesn't override it

projects:
  - name: api-service
    path: /Users/username/code/my-api
    command_args:
      template: api-template       # overrides the 'template' default for this project
```

**Reserved keys** — the following built-in keys cannot be reused: `q`, `ctrl+c`, `esc`, `/`, `up`, `down`, `k`, `j`, `enter`, `o`, `space`, `right`, `left`, `l`, `h`, `n`, `a`, `d`, `c`, `t`, `v`, `i`, `e`, `r`. A config that reuses any of them is rejected at startup with a clear error.

After a custom command exits, the affected project's worktrees reload automatically so `*` / `↑N` / `↓N` annotations reflect any git state changes.

## Git Status Indicators

In the TUI, each worktree displays status indicators:

- **●**: Worktree has uncommitted changes
- **↑N**: N commits ahead of upstream
- **↓N**: N commits behind upstream
- **[branch]**: Current branch name
- **Primary**: Indicates the primary/main worktree

## How It Works

`wt` uses `git worktree list` to discover all worktrees for each configured project. It parses the output to display:

- Worktree path
- Current branch
- Git status (changes, ahead/behind counters)
- Primary worktree indicator

When you create a new worktree, it's placed in a sibling directory to the project with the branch name as the folder name (e.g., `/path/to/project/../feature-branch`).

## Development

### Building

```bash
go build
```

This creates the `wt` binary in the current directory.

### Running Tests

```bash
go test -v ./...
```

Or with coverage:

```bash
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Formatting Code

```bash
go fmt ./...
```

### Linting

```bash
golangci-lint run
```

Install golangci-lint: https://golangci-lint.run/welcome/install/

## Requirements

- Go 1.25 or later
- Git 2.5 or later (for worktree support)
- VS Code (optional, for 'o' command integration)

## Troubleshooting

### "No projects configured" message

Run `wt config init` to create the config file, then add projects with `wt config add`.

### VS Code doesn't open

Ensure the `code` command is available in your PATH. On macOS, open VS Code and run `Shell Command: Install 'code' command in PATH` from the command palette.

### Worktree creation fails

Ensure you have write permissions in the parent directory of your project, and that the branch name doesn't already exist.

## License

MIT

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on GitHub.

## Author

Created by [bmingles](https://github.com/bmingles)
