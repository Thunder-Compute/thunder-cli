# Thunder Compute CLI

`tnr` is the CLI for [Thunder Compute](https://www.thundercompute.com), a cloud GPU platform for AI/ML workloads.

## MCP Server

Thunder Compute provides a remote MCP server that lets AI coding agents manage GPU instances directly — no local install required.

**Connect it:**

| Agent | Setup |
|-------|-------|
| Claude Code | `/mcp add --transport http https://api.thundercompute.com:8443/mcp` |
| Cursor | Add to `.cursor/mcp.json` (see config below) |
| Windsurf | Add to MCP settings (see config below) |
| Other agents | POST to endpoint with OAuth 2.0 |

```json
{
  "mcpServers": {
    "thunder-compute": {
      "type": "http",
      "url": "https://api.thundercompute.com:8443/mcp"
    }
  }
}
```

No API tokens needed — authenticates via OAuth in the browser.

**28 tools:** instance management, GPU specs/pricing/availability, snapshots, SSH keys, port forwarding, billing, and API tokens.

[Full MCP documentation](https://www.thundercompute.com/docs/guides/mcp-server)

## CLI Quick Reference

```
tnr login            # Authenticate with Thunder Compute
tnr create           # Create a GPU instance
tnr status           # List instances with status
tnr connect <id>     # SSH into an instance
tnr scp <src> <dst>  # Transfer files
tnr delete <id>      # Delete an instance
tnr port <id> <port> # Forward a port
```

Instance IDs are integers. Add `--json` for scripted/non-interactive usage.

## Development

- **Language:** Go 1.25
- **Build:** `go build -o tnr`
- **Test:** `go test ./...`

## Links

- [Documentation](https://www.thundercompute.com/docs)
- [CLI Reference](https://www.thundercompute.com/docs/cli-reference)
- [API Reference](https://www.thundercompute.com/docs/api-reference)
- [Quick Start](https://www.thundercompute.com/docs/quickstart)
