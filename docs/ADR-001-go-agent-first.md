# ADR-001: Go, agent-first CLI

- Status: accepted
- Date: 2026-02-04

## Context
We need a standalone CLI for Sure that is primarily used by agents (non-interactive), with deterministic outputs and minimal install friction.

## Decision
Implement `sure-cli` in Go.

## Rationale
- Single binary distribution (good for self-hosted/server environments)
- Fast startup and low runtime dependencies
- Easy structured JSON output and stable schemas
- Mature CLI libraries (Cobra/Viper)

## Consequences
- Contributors need Go toolchain
- TUI is not a priority for v0.1
