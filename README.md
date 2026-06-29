# chaos-sloth

Chaos engineering for Proxmox. It sloooowly kills k8s nodes in my proxmox env. It periodically targets a random VM from a configured list and performs a disruptive action (hibernate, pause, stop, or reset), then resumes it automatically.

Inspired by [chaoskube](https://github.com/linki/chaoskube), but for Proxmox VMs instead of Kubernetes pods.

## How it works

1. Wait for the configured interval (with optional jitter)
2. Pick a random VM from the target list
3. Perform the configured action
4. For `hibernate` and `pause`: automatically resume after `resume_after` (default 5m)
5. Repeat

## Actions

| Action | Proxmox UI equivalent | Effect |
|---|---|---|
| `hibernate` | Hibernate | Saves VM state to disk, stops the VM, frees host RAM |
| `pause` | Pause | Freezes VM in RAM, host RAM remains occupied |
| `stop` | Stop | Graceful ACPI shutdown, VM stays off until manually started |
| `reset` | Reset | Hard reset, no graceful shutdown |
| `random` | — | Randomly picks one of the above each time |

## Configuration

All settings can be provided via environment variables or a YAML config file. Environment variables always take precedence.

### Environment variables

| Variable | Description |
|---|---|
| `PROXMOX_URL` | Proxmox API URL, e.g. `https://proxmox.example.com:8006` |
| `PROXMOX_TOKEN_ID` | API token ID, e.g. `user@pam!chaos-sloth` |
| `PROXMOX_TOKEN_SECRET` | API token secret |
| `PROXMOX_INSECURE_TLS` | Set to `true` to skip TLS verification |
| `CHAOS_TARGETS` | JSON array of targets: `[{"node":"pve1","vmid":101,"name":"web-1"}]` |
| `CHAOS_ACTION` | `hibernate` \| `pause` \| `stop` \| `reset` \| `random` (default: `hibernate`) |
| `CHAOS_RESUME_AFTER` | Duration to wait before resuming, e.g. `5m` (default: `5m`) |
| `CHAOS_INTERVAL` | Interval between chaos events, e.g. `30m` |
| `CHAOS_JITTER` | Random variation on the interval in percent, e.g. `20` for ±20% |
| `CHAOS_DRY_RUN` | Set to `true` to log actions without calling the API |

### Config file (optional)

For local use you can use a YAML file instead. Copy the example and fill in your values:

```bash
cp config.example.yaml config.yaml
./chaos-sloth -config config.yaml
```

### Proxmox API token

Create a token in the Proxmox UI under **Datacenter → Permissions → API Tokens**. The token needs the `VM.PowerMgmt` privilege on the target VMs.

## Running

### Binary

```bash
export PROXMOX_URL=https://proxmox.example.com:8006
export PROXMOX_TOKEN_ID=user@pam!chaos-sloth
export PROXMOX_TOKEN_SECRET=your-secret
export CHAOS_TARGETS='[{"node":"pve1","vmid":101,"name":"web-1"}]'
export CHAOS_INTERVAL=30m
./chaos-sloth
```

### Docker

```bash
docker run --rm \
  -e PROXMOX_URL=https://proxmox.example.com:8006 \
  -e PROXMOX_TOKEN_ID=user@pam!chaos-sloth \
  -e PROXMOX_TOKEN_SECRET=your-secret \
  -e CHAOS_TARGETS='[{"node":"pve1","vmid":101,"name":"web-1"}]' \
  -e CHAOS_INTERVAL=30m \
  ghcr.io/yoramvandevelde/chaos-sloth:latest
```

### Kubernetes (Helm)

```bash
helm install chaos-sloth ./chart \
  --set proxmox.url=https://proxmox.example.com:8006 \
  --set proxmox.tokenId=user@pam!chaos-sloth \
  --set proxmox.tokenSecret=your-secret \
  --set chaos.interval=30m \
  --set-json targets='[{"node":"pve1","vmid":101,"name":"web-1"}]'
```

Or with a `values.yaml`:

```yaml
proxmox:
  url: "https://proxmox.example.com:8006"
  tokenId: "user@pam!chaos-sloth"
  tokenSecret: "your-secret"  # or use existingSecret

targets:
  - node: pve1
    vmid: 101
    name: web-1
  - node: pve2
    vmid: 201
    name: worker-1

chaos:
  action: "hibernate"
  interval: "30m"
  jitter: 20
```

```bash
helm install chaos-sloth ./chart -f values.yaml
```

To use an existing Kubernetes secret for the token:

```bash
kubectl create secret generic proxmox-token --from-literal=token-secret=your-secret
helm install chaos-sloth ./chart -f values.yaml --set proxmox.existingSecret=proxmox-token
```

## Building

```bash
make build
```
