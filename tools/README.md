# Okteto Tools

Internal tools used by Okteto CLI during development workflows.

## Tools

### remote

SSH server for remote development containers. Provides secure remote access to development environments.

**Source**: https://github.com/okteto/remote

### supervisor

Process supervisor for development containers. Monitors and manages application processes during development.

**Source**: https://github.com/okteto/supervisor

### clean

Cleanup utility for development containers. Manages cleanup operations in remote environments.

**Source**: https://github.com/okteto/clean

## Building

Build all tools:

```bash
make build
```

Build individual tools:

```bash
make remote
make supervisor
make clean-tool
```

Binaries are output to `../bin/`.

## Testing

Run tests:

```bash
make test
```

Run linter:

```bash
make lint
```

## Development

Update dependencies:

```bash
make dep
```

## Docker

These tools are built automatically as part of the main Okteto CLI Docker image. See the root `Dockerfile` for details.
