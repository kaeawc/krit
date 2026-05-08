# MCP server setup

Krit ships a Model Context Protocol server, `krit-mcp`, that lets AI
agents like Claude Code, Claude Desktop, and Codex call krit's analysis
directly. The server speaks newline-delimited JSON-RPC 2.0 over stdio,
which is what every MCP-compatible client expects.

## What it gives the agent

Once configured, the agent gets seven tools backed by the same engine
the CLI uses:

| Tool | What it does |
|------|--------------|
| `analyze` | Run rules on a code buffer, an entire project, an Android module, or to estimate the noise of enabling a specific rule |
| `fix` | Suggest auto-fixes (cosmetic / idiomatic / semantic) or generate `@Suppress` annotations |
| `rules` | Explain a rule, full-text search by concept, list categories, or generate `krit.yml` for a rule |
| `metrics` | Query rule-level finding count history written by `krit metrics log` |
| `symbols` | Cross-file references and per-file outline |
| `types` | Type queries: classes, hierarchy, imports, sealed variants, enum entries, function signatures |
| `structure` | Project structure: Gradle modules, framework profile, hotspots, breadth, package drift |

It also publishes a `krit://rules` resource and a few prompts
(`review_kotlin`, `prepare_pr`, `refactor_check`) that bundle multiple
tool calls.

## First, locate the binary

After `brew install --cask kaeawc/tap/krit` or `curl … | sh`, find
where `krit-mcp` ended up:

```bash
command -v krit-mcp
```

Use that absolute path in the configurations below — MCP launchers
don't always inherit your shell's `PATH`, especially desktop apps, so
"works in terminal, doesn't work in Claude Desktop" is a common
footgun. Absolute paths sidestep it.

Typical paths:

- `/opt/homebrew/bin/krit-mcp` — Apple Silicon Homebrew
- `/usr/local/bin/krit-mcp` — Intel Homebrew
- `/home/linuxbrew/.linuxbrew/bin/krit-mcp` — Linuxbrew
- `~/.local/bin/krit-mcp` — `install.sh` default
- `$(go env GOPATH)/bin/krit-mcp` — `go install`

## Claude Code

Edit `~/.claude.json` (the file Claude Code creates on first run):

```json
{
  "mcpServers": {
    "krit": {
      "type": "stdio",
      "command": "/opt/homebrew/bin/krit-mcp",
      "args": [],
      "env": {}
    }
  }
}
```

Or use the CLI to edit it for you:

```bash
claude mcp add krit /opt/homebrew/bin/krit-mcp
```

Inside a Claude Code session, `/mcp` shows current connections;
restarting reconnects.

## Claude Desktop

`~/Library/Application Support/Claude/claude_desktop_config.json` on
macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows,
`~/.config/Claude/claude_desktop_config.json` on Linux:

```json
{
  "mcpServers": {
    "krit": {
      "command": "/opt/homebrew/bin/krit-mcp"
    }
  }
}
```

Quit Claude Desktop and reopen it — config is read once at startup.
Tools appear in the slash-command picker.

## Codex (OpenAI Codex CLI)

`~/.codex/config.toml`:

```toml
[mcp_servers.krit]
command = "/opt/homebrew/bin/krit-mcp"
args = []
```

Restart `codex`; MCP servers connect on startup.

## Generic MCP client

Any MCP-compatible client should accept this shape:

- **Transport**: stdio
- **Command**: absolute path to `krit-mcp`
- **Args**: none required; pass `-v` for verbose lifecycle logging on stderr
- **Working directory**: doesn't matter; krit-mcp tools take explicit paths

## Verifying it works

The fastest test that doesn't need a client is to send two JSON-RPC
messages directly:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | krit-mcp
```

You should see two single-line JSON responses on stdout: one for
`initialize` (with `serverInfo.name = "krit-mcp"`) and one listing the
seven tools. If you see nothing, the server is probably waiting for
LSP-style framing — make sure you're on a krit version that includes
the NDJSON transport (any released build does).

## Troubleshooting

**Tools don't appear in the client.** The client probably failed to
spawn `krit-mcp`. Check the client's logs (Claude Desktop: `Help →
Show Logs`, Claude Code: stderr of the host terminal). Re-run the
verification snippet above to rule out a krit problem.

**"Command not found" when the path looks right.** GUI clients on
macOS launch from a minimal environment that doesn't include
`/opt/homebrew/bin` on `PATH`. Use the absolute path in the config.

**The server starts but every tool returns an error about a path.**
The agent is calling tools with relative paths. Pass absolute paths
in the agent's prompt or tool arguments.

**Verbose logging.** Add `"args": ["-v"]` to the config. The server
emits one line per request to stderr without affecting the JSON-RPC
stdout stream.
