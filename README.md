# acp-issue-analyzer

> **WARNING: This project is experimental and has been vibe-coded without thorough review. Use at your own risk. Expect rough edges, bugs, and breaking changes.**

A terminal UI tool that uses AI agents (via the [Agent Client Protocol](https://agentclientprotocol.com/get-started/introduction)) to automatically analyze GitHub issues and pull requests.

This project serves as a demonstration of the utility of ACP -- because agents communicate over a standard protocol, you can easily swap different agents in and out of workflows without changing any application code.

## What it does

- Browse GitHub issues and PRs from configured repositories
- Launch AI agents to investigate bugs, evaluate feature requests, or review PRs
- Agents run in isolated git worktrees so they can safely explore code without affecting your working tree
- Run multiple agents in parallel with real-time progress tracking
- Review completed analysis sessions

## Building

```
make build
```

Or directly:

```
go build -o bin/acp-issue-analyzer ./cmd/acp-issue-analyzer
```

## Configuration

Copy `config.example.toml` to `~/.config/acp-issue-analyzer/config.toml` and edit it to configure:

- GitHub repositories to monitor
- AI agents to use (e.g. Claude CLI with `--acp`)
- Label filters and safety policies

## Usage

```
./bin/acp-issue-analyzer [path/to/config.toml]
```

If no config path is given, it defaults to `~/.config/acp-issue-analyzer/config.toml`.

Session data and logs are stored in `~/.local/share/acp-issue-analyzer/`.
