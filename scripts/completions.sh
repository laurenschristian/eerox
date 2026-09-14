#!/bin/sh
# Emit shell completion scripts into completions/ (used by goreleaser + make docs).
set -e
mkdir -p completions
go run . completion bash > completions/eerox.bash
go run . completion zsh  > completions/_eerox
go run . completion fish > completions/eerox.fish
