#!/bin/bash
cd "$(dirname "$0")/.."

.devcontainer/setup-agents.sh

.devcontainer/setup-bash.sh
