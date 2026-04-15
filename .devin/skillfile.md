# Thunder Compute

Use this skill when the user asks to create, manage, or connect to cloud GPU instances on Thunder Compute.

## When to use

- User wants to provision or manage cloud GPU instances
- User asks about GPU availability, pricing, or specs
- User needs to create snapshots, forward ports, or transfer files to/from a GPU instance
- User asks about Thunder Compute specifically

## Setup

### Remote (no local install required)

Connect to the hosted MCP server with OAuth authentication:

```json
{
  "mcpServers": {
    "thunder-compute": {
      "type": "http",
      "url": "https://thundercompute.com/mcp"
    }
  }
}
```

Your agent will walk you through authentication automatically via OAuth.

## Intent Routing

Match the user's intent to the right action:

| Intent | Action |
|--------|--------|
| Create/launch/spin up an instance | **Create workflow** below |
| List/show/check instances or status | Call `list_instances` |
| Delete/destroy/tear down | **Delete workflow** below |
| Modify/change/resize | Call `modify_instance` with the requested changes |
| Run a command on my instance | Call `run_command` with instance_id and command |
| What GPUs are available / specs | Call `get_specs` |
| Which GPUs are available right now | Call `get_availability` |
| How much does it cost / pricing | Call `get_pricing` |
| Templates / what OS options | Call `list_templates` |
| Snapshot / backup | **Snapshot workflow** below |
| SSH keys (org-level) | Call `list_ssh_keys`, `create_ssh_key`, or `delete_ssh_key` |
| Add SSH key to a running instance | Call `add_ssh_key_to_instance` with instance_id and public_key |
| Forward/expose port | Call `forward_port` with instance_id and ports |
| Remove port forwarding | Call `delete_port` with instance_id and ports |
| List forwarded ports | Call `list_ports` |
| Connect / SSH into instance | Call `get_ssh_command`, show the command to run |
| Copy files / SCP | Call `get_scp_command`, show the command to run |
| How much have I spent / usage | Call `get_meter_data` with start_time and end_time |
| Show my invoices / billing | Call `get_upcoming_invoice` or `get_invoice_history` |
| What's my subscription / plan | Call `get_subscription` |
| API tokens / create token | Call `list_tokens`, `create_token`, or `delete_token` |

## Workflows

### Create

1. If no GPU type specified, call `get_specs` and `get_pricing` to help choose
2. If no template specified, call `list_templates` — suggest `ubuntu22.04-cuda12.4` as default
3. Call `create_instance` with `gpu_type`, `template`, `mode` ("prototyping" unless user said "production"), `num_gpus` (default 1), `disk_size_gb` (default 100)
4. Report the instance UUID; user can connect via `tnr connect` once running

### Delete

1. If no instance ID given, call `list_instances` to help identify which one
2. **Always confirm before deleting** — state the instance name/ID and that deletion is irreversible
3. Call `delete_instance`

### Snapshot

1. Creating: call `create_snapshot` with instance_id and name
2. Listing: call `list_snapshots`
3. Deleting: confirm first (irreversible), then call `delete_snapshot`

## Defaults

- Mode: `prototyping` (auto-stops when idle, cheaper)
- Disk: 100 GB
- GPUs: 1
- Template: `ubuntu22.04-cuda12.4`

## Safety Rules

- **Never delete without explicit user confirmation**
- **Warn about production mode** — it runs 24/7 and bills continuously
- If auth fails, tell the user to re-authenticate
- If the requested GPU type isn't available, show alternatives from `get_specs`

## Available MCP Tools

### Instance Management
- `list_instances` — List all instances
- `create_instance` — Create instance (gpu_type, template, mode, num_gpus, vcpus, disk_size_gb, ssh_key_name)
- `delete_instance` — Delete instance (instance_id)
- `modify_instance` — Modify instance (instance_id + optional fields)
- `run_command` — Execute shell command on instance (instance_id, command, timeout)

### Information
- `get_specs` — Available GPU specifications
- `get_availability` — Current GPU availability per spec
- `get_pricing` — Per-hour GPU pricing
- `list_templates` — OS templates

### Snapshots
- `list_snapshots` — List snapshots
- `create_snapshot` — Create snapshot (instance_id, name)
- `delete_snapshot` — Delete snapshot (snapshot_id)

### SSH Keys
- `list_ssh_keys` — List SSH keys
- `create_ssh_key` — Add SSH key to org (name, public_key)
- `delete_ssh_key` — Delete SSH key (key_id)
- `add_ssh_key_to_instance` — Add SSH key to a running instance (instance_id, public_key)

### Port Forwarding
- `list_ports` — List all instances with forwarded ports
- `forward_port` — Forward HTTP ports (instance_id, ports)
- `delete_port` — Remove forwarded ports (instance_id, ports)

### Connectivity
- `get_ssh_command` — Get SSH command to connect (instance_id, tunnel_ports)
- `get_scp_command` — Get SCP command to transfer files (instance_id, direction, local_path, remote_path)

### Billing & Usage
- `get_meter_data` — GPU usage metrics (start_time, end_time, grouping)
- `get_upcoming_invoice` — Estimated charges for current billing period
- `get_invoice_history` — Historical invoices (limit)
- `get_subscription` — Subscription plan, status, and payment info

### API Tokens
- `list_tokens` — List named API tokens
- `create_token` — Create API token (name)
- `delete_token` — Delete API token (name)

## CLI Quick Reference

```
tnr login            # Authenticate
tnr create           # Create GPU instance
tnr status           # List instances
tnr connect <id>     # SSH into instance
tnr scp <src> <dst>  # File transfer
tnr delete <id>      # Delete instance
```

## Links

- [Full docs](https://www.thundercompute.com/docs)
- [MCP server guide](https://www.thundercompute.com/docs/guides/mcp-server)
- [CLI reference](https://www.thundercompute.com/docs/cli-reference)
- [API spec](https://api.thundercompute.com:8443/openapi.json)
