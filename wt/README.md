# wt - Git Worktree Manager

A command-line tool for managing Git worktrees with an interactive TUI interface.

## Features

- **Interactive TUI**: Navigate through your projects and worktrees with an intuitive interface
- **Quick VS Code Integration**: Open any worktree in VS Code with a single keystroke
- **Worktree Management**: Create and delete worktrees directly from the TUI
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
  - name: api
    path: /Users/username/code/api-service
  - name: frontend
    path: /Users/username/code/frontend-app
```

Each project has:
- **name**: A unique identifier for the project
- **path**: Absolute path to the Git repository

See [example-config.yaml](example-config.yaml) for a complete example.

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
