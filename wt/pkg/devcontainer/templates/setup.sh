#!/bin/bash
cd "$(dirname "$0")/.."

sudo chown -R vscode:vscode $PROJECT_PATH/node_modules

.devcontainer/setup-agents.sh

.devcontainer/setup-bash.sh
