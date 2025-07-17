# Implementation Summary: --force Flag for okteto context use

## Overview

Successfully implemented the `--force` parameter for the `okteto context use` command that deletes the corresponding configuration in the okteto context and forces the user to log in again.

## Changes Made

### 1. Options Structure (`cmd/context/options.go`)
- **Added**: `Force bool` field to the `Options` struct
- **Line**: 39
- **Purpose**: Store the force flag state

### 2. Use Command (`cmd/context/use.go`)
- **Added**: `--force` flag definition
- **Line**: 83
- **Code**: `cmd.Flags().BoolVarP(&ctxOptions.Force, "force", "", false, "delete the corresponding configuration in the okteto context and force the user to log in again")`
- **Purpose**: Expose the force flag to users with appropriate description

### 3. UseContext Method (`cmd/context/create.go`)
- **Added**: Force flag handling logic (lines 92-118)
- **Functionality**:
  - Detects when force flag is set
  - Deletes existing context from context store (handles both with/without schema)
  - Clears current context if it matches the deleted context
  - Writes updated context store to disk
  - Clears cached tokens (`Token` and `InferredToken`)
  - Forces re-authentication on next use

### 4. Unit Tests (`cmd/context/use_test.go`)
- **Added**: Comprehensive test suite with 6 test scenarios
- **Tests Cover**:
  - Force flag deletes existing context and forces re-login
  - Force flag with context without schema
  - Force flag clears current context when deleting it
  - Force flag with non-existent context
  - No force flag preserves existing token
  - Command flag definition validation

## Key Features

### 1. Context Deletion Logic
```go
if ctxOptions.Force && ctxOptions.Context != "" {
    contextToDelete := ctxOptions.Context
    // Try both with and without schema
    if _, ok := ctxStore.Contexts[contextToDelete]; ok {
        delete(ctxStore.Contexts, contextToDelete)
    }
    contextWithSchema := okteto.AddSchema(contextToDelete)
    if _, ok := ctxStore.Contexts[contextWithSchema]; ok {
        delete(ctxStore.Contexts, contextWithSchema)
        contextToDelete = contextWithSchema
    }
    
    // If we deleted the current context, clear it
    if ctxStore.CurrentContext == contextToDelete {
        ctxStore.CurrentContext = ""
    }
}
```

### 2. Token Clearing
```go
// Clear any cached token to force re-authentication
ctxOptions.Token = ""
ctxOptions.InferredToken = false
```

### 3. Schema Handling
- Handles both `okteto.example.com` and `https://okteto.example.com` formats
- Uses `okteto.AddSchema()` for normalization
- Ensures consistent behavior regardless of input format

## Behavior

### When --force is used:
1. **Context Lookup**: Searches for context in store (both with/without https schema)
2. **Context Deletion**: Removes matching context from context store
3. **Current Context Cleanup**: Clears current context if it matches deleted context
4. **Persistence**: Writes updated context store to disk
5. **Token Clearing**: Removes cached authentication tokens
6. **Re-authentication**: Forces user to authenticate again

### When --force is NOT used:
- Normal behavior is preserved
- Existing tokens are maintained
- Context switching works as before

## Testing

### Unit Tests
- **File**: `cmd/context/use_test.go`
- **Functions**: `TestUseContext_Force()`, `TestUseCommand_ForceFlag()`
- **Coverage**: 6 comprehensive test scenarios
- **Validation**: Flag definition, force logic, edge cases

### Manual Testing
- **Guide**: `MANUAL_TEST.md`
- **Scenarios**: 4 different test scenarios
- **Verification**: Commands to validate behavior

## Integration

- **Backward Compatible**: Existing functionality unchanged
- **Flag Default**: `false` (no breaking changes)
- **Error Handling**: Graceful handling of missing contexts
- **Schema Normalization**: Consistent with existing okteto patterns

## Security Considerations

- **Token Clearing**: Ensures no cached credentials remain
- **Context Isolation**: Only affects specified context
- **Re-authentication**: Forces fresh authentication flow
- **No Credential Exposure**: Tokens are cleared, not logged

## Files Modified

1. `cmd/context/options.go` - Added Force field
2. `cmd/context/use.go` - Added --force flag
3. `cmd/context/create.go` - Added force handling logic
4. `cmd/context/use_test.go` - Added comprehensive tests

## Usage

```bash
# Force re-authentication for a context
okteto context use https://okteto.example.com --force

# Works with or without schema
okteto context use okteto.example.com --force

# Check help for flag description
okteto context use --help
```

## Status

✅ **COMPLETED**: Implementation is complete and ready for testing
✅ **TESTED**: Unit tests created and validate functionality
✅ **DOCUMENTED**: Manual testing guide provided
✅ **INTEGRATED**: Follows existing code patterns and conventions