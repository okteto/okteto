# Okteto Snapshot Command

The `okteto snapshot` command provides functionality to manage volume snapshots for your development environment.

## Subcommands

### `okteto snapshot upload`

Uploads local data to a volume snapshot by performing the following steps:

1. **Calculate Directory Size**: Analyzes the local directory to determine storage requirements
2. **Create PVC**: Creates a Persistent Volume Claim with appropriate size (calculated size + 20% overhead)
3. **Create Temporary Pod**: Deploys a temporary pod with the PVC attached
4. **Copy Data**: Uses `kubectl cp` to transfer data from local to the pod with progress tracking
5. **Create Snapshot**: Creates a VolumeSnapshot resource following Kubernetes snapshot.storage.k8s.io/v1 API
6. **Wait for Ready**: Monitors the snapshot until it's ready for use
7. **Cleanup**: Removes temporary resources (pod and PVC)

#### Usage

```bash
okteto snapshot upload [LOCAL_PATH] [flags]
```

#### Flags

- `--size`: Override the calculated size for the PVC (e.g., "10Gi")
- `--name`: Custom name for the snapshot (default: auto-generated)

#### Examples

```bash
# Upload current directory with auto-calculated size
okteto snapshot upload .

# Upload specific directory with custom size
okteto snapshot upload /path/to/data --size 5Gi

# Upload with custom snapshot name
okteto snapshot upload ./data --name my-snapshot
```

#### Output

Upon successful completion, the command returns:
- Namespace where the snapshot was created
- Snapshot name for future reference

## Requirements

- Active Okteto context with valid namespace
- `kubectl` available in PATH
- Volume snapshot support in the cluster
- Appropriate RBAC permissions for PVC and VolumeSnapshot operations

## Volume Snapshot Integration

This command follows the [Okteto Volume Snapshots guide](https://www.okteto.com/docs/core/use-volume-snapshots/#using-volume-snapshots-in-your-development-environment) and creates snapshots compatible with the Kubernetes Volume Snapshot API.