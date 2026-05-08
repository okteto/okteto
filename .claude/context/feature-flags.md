# Feature Flags

> **See also**: [conventions.md](conventions.md) for general Go conventions and [architecture.md](architecture.md) for package structure.

Feature flags and the NoOp strategy are related but distinct concepts. A feature flag splits execution into different code paths. The NoOp strategy is a specific way to implement one of those paths. They often appear together but do not always.

---

## Feature Flags — Splitting Code Paths

A feature flag introduces at least two code paths. Rather than scattering the same `if flag {…}` check everywhere, model each path as a strategy and create the right one once at the boundary.

```go
// createStrategy reads the flag once and returns the appropriate implementation.
func createStrategy() strategyInterface {
    if env.LoadBoolean(MyFeatureEnvVar) {
        return newBehavior{}
    }
    return legacyBehavior{}
}

// Callers never check the flag — they just use the strategy.
strategy := createStrategy()
strategy.DoWork()
```

### Why the strategy pattern

- The flag is read in a single place — easier to remove later.
- Callers stay oblivious to which implementation is active.
- Each strategy is independently testable.

### Flag values are not always boolean

Flags can carry integer, string, or enum values when the behavior has more than two variants:

```go
const MyFeatureEnvVar string = "OKTETO_MY_FEATURE"

func createStrategy() strategyInterface {
    switch env.LoadOrDefault(MyFeatureEnvVar, "v1") {
    case "v2":
        return v2Behavior{}
    case "v3":
        return v3Behavior{}
    default:
        return v1Behavior{}
    }
}
```

Use `env.LoadBooleanOrDefault`, `env.LoadIntOrDefault`, or raw `os.Getenv` / `env.LoadOrDefault` as appropriate.

### Defaults are context-dependent

The default value should reflect what is safe **right now**, not a blanket rule:

- A brand-new, unvalidated feature → default to the safe/existing path (often `false` or the old variant).
- A feature that has been promoted and is now the stable path → default to the new path (often `true` or the new variant).
- A flag being used to disable a broken path → default to `true` (enabled = stable).

Document _why_ the default was chosen next to the constant.

---

## The NoOp Strategy

The NoOp strategy is a specific application of the strategy pattern for the case where the **existing behavior is to do nothing**: instead of a conditional around the call site, the inactive path is a struct whose methods do nothing, preserving current observable behavior.

**Without NoOp (avoid):**

```go
if featureFlagEnabled {
    callEnabledBehavior()
}
```

**With NoOp (preferred):**

```go
func createStrategy() strategyInterface {
    if flagIsActive() {
        return newBehavior{}
    }
    return noOpBehavior{} // satisfies the interface, all methods are no-ops
}

// Call site is clean — no conditional needed.
strategy := createStrategy()
strategy.Behavior()
```

Use the NoOp strategy when:

- The existing behavior is to do nothing — the feature adds something new that was previously absent.
- The flag controls an optional side effect (notifications, metrics emission, etc.).
- There are many call sites that would otherwise each need an `if` guard.

Do **not** force a NoOp strategy when:

- The feature replaces existing behavior rather than adding to it — use two real implementations instead.
- There is only one call site — a simple `if` is clearer.

---

## Rollout Lifecycle

1. **Add flag (inactive default)**: New code path exists but does nothing new — a structural NoOp or a pass-through to existing logic.
2. **Implement behind the flag**: Develop and test the new behavior with the flag enabled.
3. **Flip default**: Only after the implementation is proven stable in production.
4. **Remove flag**: Once fully rolled out, delete the flag and the old code path.

---

## Implementation Pattern

### 1. Define the flag constant

Place it near the top of the package file that owns the behavior. Use the `OKTETO_` prefix.

```go
// OktetoMyFeatureEnvVar enables the new scheduling algorithm.
// Default preserves existing behavior: unset means the legacy path runs.
const OktetoMyFeatureEnvVar string = "OKTETO_MY_FEATURE"
```

### 2. Load the flag

```go
import "github.com/okteto/okteto/pkg/env"

// Boolean flag, inactive by default:
env.LoadBoolean(OktetoMyFeatureEnvVar)           // false when unset
env.LoadBooleanOrDefault(OktetoMyFeatureEnvVar, false) // explicit

// Boolean flag, active by default (promoted feature):
env.LoadBooleanOrDefault(OktetoMyFeatureEnvVar, true)

// Integer flag:
env.LoadIntOrDefault(OktetoMyFeatureEnvVar, 3)
```

---

## Helper Functions (`pkg/env`)

| Function                             | Behavior when unset | Use for                                      |
| ------------------------------------ | ------------------- | -------------------------------------------- |
| `env.LoadBoolean(k)`                 | Returns `false`     | Boolean flags defaulting to inactive         |
| `env.LoadBooleanOrDefault(k, false)` | Returns `false`     | Boolean flags with explicit inactive default |
| `env.LoadBooleanOrDefault(k, true)`  | Returns `true`      | Boolean flags defaulting to active           |
| `env.LoadIntOrDefault(k, d)`         | Returns `d`         | Integer-valued flags                         |

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

    // Default is true because the port-forwarder is now the stable production
    // path. It was introduced with default false and promoted once validated.
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
- [ ] Comment explains the flag's purpose and justifies the chosen default
- [ ] Flag is read once; callers receive a strategy, not the raw flag value
- [ ] New code path preserves existing behavior when the flag keeps its default (NoOp or pass-through)
- [ ] Tests cover each code path independently
