# Manual Testing Guide for --force Flag

This document provides instructions for manually testing the new `--force` flag functionality in the `okteto context use` command.

## What the --force flag does

When the `--force` flag is used with `okteto context use`, it:
1. Deletes the corresponding configuration from the okteto context store
2. Clears any cached authentication tokens
3. Forces the user to log in again when using that context

## Test Scenarios

### Test 1: Basic Force Flag Functionality

1. **Setup**: Ensure you have an existing Okteto context configured
   ```bash
   okteto context list
   ```

2. **Test the flag exists**:
   ```bash
   okteto context use --help
   ```
   - Verify that `--force` flag is listed in the help output
   - Verify the description: "delete the corresponding configuration in the okteto context and force the user to log in again"

3. **Use context with force flag**:
   ```bash
   okteto context use <your-context-url> --force
   ```
   - This should delete the existing context configuration
   - Should prompt for re-authentication

### Test 2: Force Flag with Non-existent Context

1. **Test with a context that doesn't exist**:
   ```bash
   okteto context use https://non-existent.okteto.com --force
   ```
   - Should work normally (no error about missing context)
   - Should create the context as usual

### Test 3: Force Flag Clears Current Context

1. **Setup**: Set a context as current
   ```bash
   okteto context use <your-context-url>
   okteto context show
   ```

2. **Use force flag on current context**:
   ```bash
   okteto context use <your-context-url> --force
   ```
   - Should clear the current context temporarily
   - Should re-establish it after successful authentication

### Test 4: Force Flag with Different URL Formats

1. **Test with URL without schema**:
   ```bash
   okteto context use your-context.okteto.com --force
   ```

2. **Test with full HTTPS URL**:
   ```bash
   okteto context use https://your-context.okteto.com --force
   ```

Both should work correctly and handle the schema normalization.

## Expected Behavior

- ✅ The `--force` flag should be available in `okteto context use --help`
- ✅ Using `--force` should delete existing context configuration
- ✅ Using `--force` should clear cached tokens
- ✅ Using `--force` should prompt for re-authentication
- ✅ Using `--force` with non-existent context should work normally
- ✅ The flag should work with both schema formats (with/without https://)

## Verification Commands

After each test, you can verify the behavior using:

```bash
# Check current context
okteto context show

# List all contexts
okteto context list

# Check if re-authentication is required
okteto namespace list
```

## Implementation Details

The implementation includes:

1. **Options struct** (`cmd/context/options.go`):
   - Added `Force bool` field

2. **Use command** (`cmd/context/use.go`):
   - Added `--force` flag with appropriate description

3. **UseContext method** (`cmd/context/create.go`):
   - Added logic to handle force flag
   - Deletes context from store when force is true
   - Clears tokens to force re-authentication
   - Handles both URL formats (with/without schema)

4. **Unit tests** (`cmd/context/use_test.go`):
   - Comprehensive test coverage for force functionality
   - Tests various scenarios including edge cases