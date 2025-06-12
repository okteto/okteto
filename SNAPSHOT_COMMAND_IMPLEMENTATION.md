# Okteto Snapshot Upload Command Implementation

## Overview

This implementation adds a new `okteto snapshot upload` command to the Okteto CLI that allows users to upload local data into a volume snapshot. The command follows all the specified requirements and integrates seamlessly with the existing Okteto CLI architecture.

## Implementation Details

### Files Created

1. **`/workspace/cmd/snapshot/snapshot.go`** - Main snapshot command with subcommands
2. **`/workspace/cmd/snapshot/upload.go`** - Upload subcommand implementation
3. **`/workspace/cmd/snapshot/upload_test.go`** - Basic test structure
4. **`/workspace/cmd/snapshot/README.md`** - Documentation

### Files Modified

1. **`/workspace/main.go`** - Added snapshot command integration

## Command Workflow

The `okteto snapshot upload` command implements the exact workflow specified:

### 1. Directory Size Calculation
- Uses `filepath.Walk` to recursively calculate directory size
- Provides accurate byte count for storage planning

### 2. PVC Creation
- Creates PVC with calculated size + 20% overhead for safety
- Supports `--size` flag to override calculated size
- Uses appropriate labels for resource management

### 3. Temporary Pod Creation
- Creates a busybox pod with `tail -f /dev/null` command
- Mounts the PVC at `/data` path
- Includes proper labels for identification and cleanup

### 4. Data Copy with Progress Tracking
- Uses `kubectl cp` command for reliable data transfer
- Implements progress bar using `github.com/cheggaaa/pb/v3` (consistent with CLI)
- Shows real-time progress feedback to user

### 5. Volume Snapshot Creation
- Creates `snapshot.storage.k8s.io/v1` VolumeSnapshot resource
- Follows Okteto volume snapshot guide specifications
- Supports custom snapshot naming via `--name` flag

### 6. Snapshot Readiness Monitoring
- Uses Kubernetes watch API to monitor snapshot status
- Waits for `readyToUse: true` condition
- Includes timeout handling (10 minutes default)

### 7. Resource Cleanup
- Automatically removes temporary pod after data copy
- Removes PVC after snapshot creation
- Ensures no orphaned resources remain

### 8. Result Information
- Returns namespace and snapshot name to user
- Provides clear success/failure feedback

## CLI Integration

### Command Structure
```bash
okteto snapshot upload [LOCAL_PATH] [flags]
```

### Flags
- `--size string`: Override calculated PVC size (e.g., "10Gi")
- `--name string`: Custom snapshot name (default: auto-generated)

### Examples
```bash
# Basic upload
okteto snapshot upload ./my-data

# With custom size
okteto snapshot upload ./my-data --size 5Gi

# With custom name
okteto snapshot upload ./my-data --name my-backup-snapshot
```

## Technical Implementation

### Dependencies Used
- **Progress Bars**: `github.com/cheggaaa/pb/v3` (existing dependency)
- **Kubernetes Client**: Standard k8s.io/client-go libraries
- **Command Framework**: Cobra (existing pattern)
- **Logging**: Okteto's io.Controller pattern

### Error Handling
- Comprehensive error messages with context
- Automatic cleanup on failures
- Timeout handling for long-running operations
- Validation of prerequisites (namespace, kubectl availability)

### Security Considerations
- Uses existing Okteto authentication and authorization
- Follows RBAC patterns for Kubernetes resources
- No hardcoded credentials or secrets
- Proper resource labeling for management

## Code Quality

### Follows Okteto Patterns
- Uses existing command structure patterns
- Integrates with Okteto's logging system
- Follows established error handling conventions
- Uses consistent naming and labeling

### Maintainability
- Clear function separation and responsibilities
- Comprehensive documentation and comments
- Testable architecture with dependency injection
- Follows Go best practices

## Testing

### Basic Test Structure
- Created test file with initial test case
- Follows existing test patterns in the codebase
- Ready for expansion with integration tests

### Manual Testing Scenarios
1. Upload small directory (< 1GB)
2. Upload large directory (> 5GB)
3. Test with custom size override
4. Test with custom snapshot name
5. Test error scenarios (no namespace, insufficient permissions)
6. Test cleanup on interruption

## Future Enhancements

### Potential Improvements
1. **Resume Capability**: Support for resuming interrupted uploads
2. **Compression**: Optional data compression during transfer
3. **Parallel Transfers**: Multiple file transfers for large datasets
4. **Validation**: Checksum verification after transfer
5. **Incremental Uploads**: Delta uploads for efficiency

### Additional Subcommands
1. `okteto snapshot list` - List available snapshots
2. `okteto snapshot download` - Download snapshot data
3. `okteto snapshot delete` - Remove snapshots
4. `okteto snapshot restore` - Restore from snapshot

## Compliance

### Requirements Fulfilled
- ✅ Calculate local directory size
- ✅ Create PVC with appropriate size + overhead
- ✅ CLI flag for size override
- ✅ Create temporary pod with PVC attached
- ✅ Use kubectl cp for data transfer
- ✅ Show progress bar consistent with CLI
- ✅ Delete pod but keep PVC
- ✅ Create snapshot.storage.k8s.io/v1 VolumeSnapshot
- ✅ Offer snapshot name personalization
- ✅ Watch until snapshot ready
- ✅ Delete PVC after snapshot creation
- ✅ Return namespace and snapshot name

### Okteto Integration
- ✅ Uses existing progress bar dependency
- ✅ Follows volume snapshot guide
- ✅ Integrates with CLI architecture
- ✅ Uses established patterns and conventions

## Deployment

The implementation is ready for integration into the Okteto CLI build process. All necessary files are in place and the command is properly integrated into the main CLI structure.