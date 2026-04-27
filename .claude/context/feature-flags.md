# Feature Flags

> **See also**: [conventions.md](conventions.md) for general Go conventions and [architecture.md](architecture.md) for package structure.

## The NoOp Strategy (Required)

**Always default new feature flags to `false`.** The new code path must be a no-operation (NoOp) — preserving existing behavior — until the implementation is proven and ready.

### Why

New features are developed incrementally. Defaulting to `false` ensures the new code path never activates for users until it has been explicitly validated and promoted. This makes feature flags safe to merge at any stage of development.

The rule: **when you add a feature flag, the flagged code path must not change observable behavior by default.** The flag is initially a safe on-ramp, not a switch that alters production behavior.

### Rollout lifecycle

1. **Add flag (default `false`)**: New code path exists but does nothing new — it either calls through to the existing logic or is a structural NoOp.
2. **Implement behind the flag**: Develop and test the new behavior with the flag enabled.
3. **Flip default to `true`**: Only after the implementation is proven stable in production.
4. **Remove flag**: Once fully rolled out, delete the flag and the old code path.

---

## Implementation Pattern

### 1. Define the flag constant

Place it near the top of the package file that owns the behavior. Use the `OKTETO_` prefix.

```go
const MyNewFeatureEnvVar string = "OKTETO_MY_NEW_FEATURE"
```

### 2. Load the flag — always default to `false`

```go
import "github.com/okteto/okteto/pkg/env"

// CORRECT: default false — opt-in, safe
if env.LoadBooleanOrDefault(MyNewFeatureEnvVar, false) {
    return newBehavior()
}
return existingBehavior()
```

**Never** default a new flag to `true`:

```go
// WRONG: default true activates new behavior for everyone immediately
if env.LoadBooleanOrDefault(MyNewFeatureEnvVar, true) { ... }
```

### 3. Use `env.LoadBoolean` when `false` is the only sensible default

`env.LoadBoolean(k)` returns `false` when the variable is unset, so it is equivalent to `LoadBooleanOrDefault(k, false)`.

```go
if env.LoadBoolean(MyNewFeatureEnvVar) {
    // new behavior
}
```

---

## Helper Functions (`pkg/env`)

| Function | Behavior when unset | Use for |
|---|---|---|
| `env.LoadBoolean(k)` | Returns `false` | New flags (always safe default) |
| `env.LoadBooleanOrDefault(k, false)` | Returns `false` | New flags with explicit default |
| `env.LoadBooleanOrDefault(k, true)` | Returns `true` | Existing flags being rolled back / disabled |
| `env.LoadIntOrDefault(k, d)` | Returns `d` | Integer-valued flags |

---

## Naming Conventions

- Environment variable: `OKTETO_<FEATURE_NAME>` — all caps, underscores
- Go constant: `Okteto<FeatureName>EnvVar` — exported `UpperCamelCase`
- Keep constants in the package that owns the feature, not in a central registry

---

## Real Example

`OktetoBuildQueueEnabledEnvVar` in `pkg/cmd/build/build.go`:

```go
const OktetoBuildQueueEnabledEnvVar string = "OKTETO_BUILD_QUEUE_ENABLED"

func GetBuildkitConnector(okCtx OktetoContextInterface, logger *io.Controller) BuildkitConnector {
    if shouldUseInClusterConnector() {
        return newInClusterConnectorWithFallback(okCtx, logger)
    }

    // Feature flag: default is true because the port-forwarder implementation
    // is now the stable production path. It was introduced with default false
    // and promoted once validated.
    if env.LoadBooleanOrDefault(OktetoBuildQueueEnabledEnvVar, true) {
        return newPortForwarderWithFallback(okCtx, logger)
    }

    return connector.NewIngressConnector(okCtx, logger)
}
```

During initial rollout this read `LoadBooleanOrDefault(..., false)`. The default was only changed to `true` once the `PortForwarder` implementation was production-ready.

---

## Checklist When Adding a Feature Flag

- [ ] Constant defined with `OKTETO_` prefix at package level
- [ ] Default is `false` — new behavior is opt-in
- [ ] New code path preserves existing behavior when flag is disabled (NoOp)
- [ ] Flag name and purpose documented in a comment next to the constant
- [ ] Tests cover both `true` and `false` paths
