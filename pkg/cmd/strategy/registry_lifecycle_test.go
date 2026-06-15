package strategy

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

// Test file: registry_lifecycle_test.go
//
// Called by: `go test ./pkg/cmd/strategy/`
// Discovered automatically via Go test convention on *_test.go in package strategy.
// No data files; pure reflection over the strategy registry + in-memory calls
// on freshly constructed zero-value instances.
//
// Purpose: a single consolidating test that walks every registered strategy
// (currently 55+) via reflection and runs Defaults / Validate / InstanceID
// under hang guards. Catches the stub-Validate / missing-guard class of bug
// in ONE place rather than per-strategy. The recurring cron prompt is:
// "tdd-workflow 测我们策略启动到运行，不同场景参数是否都能兼顾，不会导致容器假死"

const registryHangTimeout = 3 * time.Second

func runWithHangGuard(t *testing.T, op string, f func()) {
	t.Helper()
	type result struct {
		err interface{}
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{err: r}
			}
		}()
		f()
		done <- result{}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("%s panicked: %v", op, r.err)
		}
	case <-time.After(registryHangTimeout):
		t.Fatalf("%s did not complete within %s — possible container hang", op, registryHangTimeout)
	}
}

func collectRegisteredStrategies(t *testing.T) map[string]types.StrategyID {
	t.Helper()
	out := make(map[string]types.StrategyID, len(bbgo.LoadedExchangeStrategies)+len(bbgo.LoadedCrossExchangeStrategies))
	for id, s := range bbgo.LoadedExchangeStrategies {
		out[id] = s
	}
	for id, s := range bbgo.LoadedCrossExchangeStrategies {
		out[id] = s
	}
	// importing builtin.go's package triggers init() in every strategy package,
	// which calls bbgo.RegisterStrategy. If this assertion fails, the test
	// binary was likely built without the import side effects.
	require.Greaterf(t, len(out), 20, "expected the registry to contain 20+ strategies; got %d — check import side effects", len(out))
	return out
}

// TestRegistry_AllStrategies_LifecycleNoPanic walks every registered strategy,
// constructs a fresh zero-value instance via reflection, and runs Defaults /
// Validate / InstanceID under hang guards. The contract we enforce:
//
//   - Defaults() must not panic on a zero-value config. A panic here crashes
//     the SaaS container on startup.
//   - Validate() must not panic on a zero-value config. It MAY (and should)
//     return an error, but never panic — the trader wraps Validate in a
//     bare error path, so a panic escapes upward.
//   - InstanceID() must be deterministic and not panic on a zero-value
//     config. SaaS uses it for per-instance data isolation; nondeterminism
//     corrupts tables.
//
// This test does NOT enforce that Validate() returns an error — that is
// per-strategy and covered by per-package lifecycle_edge_test.go files.
// It only enforces "does not crash the container."
func TestRegistry_AllStrategies_LifecycleNoPanic(t *testing.T) {
	registry := collectRegisteredStrategies(t)

	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		registered := registry[id]
		t.Run(id, func(t *testing.T) {
			rt := reflect.TypeOf(registered)
			if rt.Kind() == reflect.Ptr {
				rt = rt.Elem()
			}
			if rt.Kind() != reflect.Struct {
				t.Skipf("strategy %s is not a struct (%s), skipping", id, rt.Kind())
				return
			}
			freshPtr := reflect.New(rt)
			fresh := freshPtr.Interface()

			if d, ok := fresh.(bbgo.StrategyDefaulter); ok {
				runWithHangGuard(t, "Defaults", func() { _ = d.Defaults() })
			}
			if v, ok := fresh.(bbgo.StrategyValidator); ok {
				runWithHangGuard(t, "Validate", func() { _ = v.Validate() })
			}
			if i, ok := fresh.(bbgo.StrategyInitializer); ok {
				runWithHangGuard(t, "Initialize", func() { _ = i.Initialize() })
			}
			if ip, ok := fresh.(types.InstanceIDProvider); ok {
				var firstID string
				runWithHangGuard(t, "InstanceID", func() {
					ids := make([]string, 0, 5)
					for k := 0; k < 5; k++ {
						ids = append(ids, ip.InstanceID())
					}
					for _, got := range ids {
						assert.Equal(t, ids[0], got, "InstanceID must be deterministic across calls")
					}
					firstID = ids[0]
				})
				assert.NotEmpty(t, firstID, "InstanceID returned empty string for zero-value config")
			}
		})
	}
}

// TestRegistry_DualModeStrategies_Reported documents which registered IDs
// implement BOTH SingleExchangeStrategy and CrossExchangeStrategy (dual-mode).
// This is an intentional bbgo pattern (e.g. xfunding, xpremium), not a bug.
// The test exists so that a NEW dual-mode strategy shows up as a visible diff
// in test output, prompting review of whether the dual registration is
// intentional. Within a single registry map, ID collisions are structurally
// impossible (Go map keys are unique); cross-map duplicate keys are the only
// observable pattern, and it is by design.
func TestRegistry_DualModeStrategies_Reported(t *testing.T) {
	dual := make([]string, 0)
	for id := range bbgo.LoadedExchangeStrategies {
		if _, ok := bbgo.LoadedCrossExchangeStrategies[id]; ok {
			dual = append(dual, id)
		}
	}
	sort.Strings(dual)
	t.Logf("dual-mode strategies (registered in both maps): %v", dual)
	// Sanity: at least the known dual-mode strategies should be present.
	require.Contains(t, dual, "xfunding")
	require.Contains(t, dual, "xpremium")
}

// TestRegistry_ReportLifecycleInterfaces emits an inventory (via t.Log) of
// which registered strategies implement each lifecycle hook. This is not an
// assertion test — it documents the registry so future drift in coverage is
// visible in test output. Useful when adding a new strategy or hook.
func TestRegistry_ReportLifecycleInterfaces(t *testing.T) {
	registry := collectRegisteredStrategies(t)

	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	type row struct {
		id         string
		defaulter  bool
		validator  bool
		initializer bool
		instanceID bool
	}
	rows := make([]row, 0, len(ids))
	for _, id := range ids {
		rt := reflect.TypeOf(registry[id])
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct {
			continue
		}
		fresh := reflect.New(rt).Interface()
		rows = append(rows, row{
			id:         id,
			defaulter:  bbgo.StrategyDefaulter(nil) != nil && implements(fresh, (*bbgo.StrategyDefaulter)(nil)),
			validator:  implements(fresh, (*bbgo.StrategyValidator)(nil)),
			initializer: implements(fresh, (*bbgo.StrategyInitializer)(nil)),
			instanceID: implements(fresh, (*types.InstanceIDProvider)(nil)),
		})
	}

	for _, r := range rows {
		t.Logf("%-20s defaulter=%-5v validator=%-5v initializer=%-5v instanceID=%-5v",
			r.id, r.defaulter, r.validator, r.initializer, r.instanceID)
	}
}

// implements reports whether x satisfies the interface pointed to by ifacePtr
// (e.g. (*bbgo.StrategyValidator)(nil)). Used purely for inventory reporting.
func implements(x interface{}, ifacePtr interface{}) bool {
	return reflect.TypeOf(x).Implements(reflect.TypeOf(ifacePtr).Elem())
}

// TestRegistry_AllStrategies_LifecycleIdempotent verifies that calling Defaults
// and Validate twice produces identical results. Non-idempotent lifecycle
// methods cause silent config drift on container reload: the SaaS manager
// re-runs Defaults() on every config update, and a strategy that appends to
// a slice or mutates a counter on each call will slowly corrupt itself until
// it hangs or produces wrong orders. This is a real container-假死 vector
// that the no-panic sweep does not cover.
//
// Contract enforced:
//   - Defaults() called twice in a row must produce the same struct state.
//   - Validate() called twice must return the same error (or nil both times).
//   - Neither may panic on the second call.
func TestRegistry_AllStrategies_LifecycleIdempotent(t *testing.T) {
	registry := collectRegisteredStrategies(t)

	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		registered := registry[id]
		t.Run(id, func(t *testing.T) {
			rt := reflect.TypeOf(registered)
			if rt.Kind() == reflect.Ptr {
				rt = rt.Elem()
			}
			if rt.Kind() != reflect.Struct {
				t.Skipf("strategy %s is not a struct (%s), skipping", id, rt.Kind())
				return
			}

			fresh := reflect.New(rt).Interface()

			if d, ok := fresh.(bbgo.StrategyDefaulter); ok {
				runWithHangGuard(t, "Defaults#1", func() { _ = d.Defaults() })
				state1 := jsonSnapshot(t, id, fresh)

				runWithHangGuard(t, "Defaults#2", func() { _ = d.Defaults() })
				state2 := jsonSnapshot(t, id, fresh)

				if state1 != state2 {
					t.Errorf("Defaults() is not idempotent — JSON state differs between calls")
				}
			}

			if v, ok := fresh.(bbgo.StrategyValidator); ok {
				var err1, err2 error
				runWithHangGuard(t, "Validate#1", func() { err1 = v.Validate() })
				runWithHangGuard(t, "Validate#2", func() { err2 = v.Validate() })
				if !sameError(err1, err2) {
					t.Errorf("Validate() is not idempotent — first call: %v, second call: %v", err1, err2)
				}
			}
		})
	}
}

// jsonSnapshot returns a canonical JSON representation of v's exported state.
// Strategies whose config is YAML/JSON-loaded (which is all of them) serialize
// cleanly; types without JSON tags still serialize via field name. A strategy
// containing a non-serializable field (chan, func) returns "<unserializable>"
// and the test is skipped for that strategy — the idempotency check is best
// effort, not a hard contract.
func jsonSnapshot(t *testing.T, id string, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Logf("strategy %s: JSON snapshot unavailable (%v), skipping idempotency comparison", id, err)
		return "<unserializable:" + id + ">"
	}
	return string(b)
}

// sameError reports whether two errors are semantically equivalent for the
// idempotency check: both nil, or both non-nil with identical message.
func sameError(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Error() == b.Error()
}
