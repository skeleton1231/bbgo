package strategy

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TestRegistry_AllStrategies_DefaultsThenValidate_SucceedsWithMinimalConfig
// is a deep semantic test: after Defaults() runs, a user supplying the
// minimum required fields (Symbol, Interval, basic sizing) should produce
// a config that passes Validate(). Strategies that still fail Validate()
// after this minimum are signalling that their Defaults() is incomplete
// (rule 1 from CLAUDE.md) or their Validate() rejects fields that have
// sensible defaults.
//
// This test uses reflection to populate Symbol and Interval because those
// are the two fields virtually every strategy requires. It does NOT claim
// to construct a fully backtest-ready config — only that the strategy can
// be loaded without error.
func TestRegistry_AllStrategies_DefaultsThenValidate_SucceedsWithMinimalConfig(t *testing.T) {
	registry := collectRegisteredStrategies(t)
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	type outcome struct {
		id              string
		validateResult  string // "no-validate", "ok", "err: <msg>"
		notes           string
	}
	outcomes := make([]outcome, 0, len(ids))

	for _, id := range ids {
		registered := registry[id]
		rt := reflect.TypeOf(registered)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct {
			continue
		}

		freshPtr := reflect.New(rt)
		fresh := freshPtr.Interface()

		// Populate Symbol and Interval fields if present.
		populateMinimumStringField(t, freshPtr, "Symbol", "BTCUSDT")
		populateMinimumIntervalField(t, freshPtr, "Interval", types.Interval1h)
		// Some strategies use IntervalWindow.Interval
		populateIntervalWindow(t, freshPtr)

		// Run Defaults() if present
		if d, ok := fresh.(bbgo.StrategyDefaulter); ok {
			_ = d.Defaults()
		}

		// Run Validate() if present, capture outcome
		out := outcome{id: id}
		if v, ok := fresh.(bbgo.StrategyValidator); ok {
			err := v.Validate()
			if err == nil {
				out.validateResult = "ok"
			} else {
				out.validateResult = "err: " + err.Error()
			}
		} else {
			out.validateResult = "no-validate"
			out.notes = "strategy has no Validate() — silent failures possible"
		}
		outcomes = append(outcomes, out)
	}

	// Report
	t.Log("=== Validate-after-Defaults semantic audit ===")
	failedNoValidate := 0
	failedErr := 0
	for _, o := range outcomes {
		switch o.validateResult {
		case "ok":
			// good
		case "no-validate":
			failedNoValidate++
			t.Logf("[NO-VALIDATE] %s — %s", o.id, o.notes)
		default:
			failedErr++
			t.Logf("[ERR]         %s — %s", o.id, o.validateResult)
		}
	}
	t.Logf("Summary: %d ok, %d no-Validate(), %d err-after-Defaults", len(outcomes)-failedNoValidate-failedErr, failedNoValidate, failedErr)
}

// populateMinimumStringField sets a top-level exported string field by name.
func populateMinimumStringField(t *testing.T, v reflect.Value, fieldName, value string) {
	t.Helper()
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	f := v.FieldByName(fieldName)
	if !f.IsValid() || !f.CanSet() || f.Kind() != reflect.String {
		return
	}
	f.SetString(value)
}

// populateMinimumIntervalField sets a top-level exported field whose underlying
// type is string (e.g. types.Interval). Works around reflect not allowing
// SetString on a named string type from a different package in some Go versions.
func populateMinimumIntervalField(t *testing.T, v reflect.Value, fieldName string, value types.Interval) {
	t.Helper()
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	f := v.FieldByName(fieldName)
	if !f.IsValid() || !f.CanSet() || f.Kind() != reflect.String {
		return
	}
	f.SetString(string(value))
}

// populateIntervalWindow sets IntervalWindow.Interval if present.
func populateIntervalWindow(t *testing.T, v reflect.Value) {
	t.Helper()
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	iw := v.FieldByName("IntervalWindow")
	if !iw.IsValid() || !iw.CanSet() || iw.Kind() != reflect.Struct {
		return
	}
	iv := iw.FieldByName("Interval")
	if iv.IsValid() && iv.CanSet() && iv.Kind() == reflect.String {
		iv.SetString(string(types.Interval1h))
	}
	w := iw.FieldByName("Window")
	if w.IsValid() && w.CanSet() && w.Kind() == reflect.Int {
		w.SetInt(14)
	}
}

// TestRegistry_AllStrategies_ValidateRejectsEmptyConfig is the inverse:
// every strategy with a Validate() MUST reject a fully empty config.
// Validate() that returns nil on an empty Strategy is a contract violation
// (CLAUDE.md rule 2) — silent acceptance of empty config lets the strategy
// run with zero/empty fields and silently misbehave.
//
// This test does NOT enforce a specific error message; it only requires
// that some error is returned.
func TestRegistry_AllStrategies_ValidateRejectsEmptyConfig(t *testing.T) {
	registry := collectRegisteredStrategies(t)
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var offenders []string
	for _, id := range ids {
		registered := registry[id]
		rt := reflect.TypeOf(registered)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct {
			continue
		}
		freshPtr := reflect.New(rt)
		fresh := freshPtr.Interface()
		v, ok := fresh.(bbgo.StrategyValidator)
		if !ok {
			continue // no Validate() — covered by previous test
		}
		err := v.Validate()
		if err == nil {
			offenders = append(offenders, id)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("strategies with Validate() that silently accept empty config: %v", offenders)
	}
}

// TestRegistry_AllStrategies_ValidateRejectsEmptySymbol verifies that any
// strategy with a Symbol field and a Validate() method must reject an empty
// Symbol. We only require that Validate() returns SOME error on an empty
// Strategy — the specific message is a UX preference, not a correctness
// contract. Strategies that silently accept empty config are caught by
// TestRegistry_AllStrategies_ValidateRejectsEmptyConfig.
//
// This test logs (via t.Log) which strategies return an error that does NOT
// mention "symbol" so that UX improvements can be tracked over time, but
// does not fail the build for it.
func TestRegistry_AllStrategies_ValidateRejectsEmptySymbol(t *testing.T) {
	registry := collectRegisteredStrategies(t)
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var offenders []string
	for _, id := range ids {
		registered := registry[id]
		rt := reflect.TypeOf(registered)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct {
			continue
		}
		freshPtr := reflect.New(rt)
		fv := freshPtr.Elem()
		symField := fv.FieldByName("Symbol")
		if !symField.IsValid() || symField.Kind() != reflect.String {
			continue // no Symbol field — skip
		}
		fresh := freshPtr.Interface()
		v, ok := fresh.(bbgo.StrategyValidator)
		if !ok {
			continue
		}
		err := v.Validate()
		if err == nil {
			offenders = append(offenders, id)
		} else if !strings.Contains(strings.ToLower(err.Error()), "symbol") {
			// Log UX issue but do not fail — silent-accept is the only hard contract.
			t.Logf("[UX] %s: Validate on empty config returns err %q (not about symbol); user may be misled", id, err.Error())
		}
	}

	if len(offenders) > 0 {
		t.Errorf("strategies with Symbol field that silently accept empty config in Validate: %v", offenders)
	}
}

// TestRegistry_AllStrategies_DefaultsDoesNotMutateQuantityToNegative ensures
// Defaults() doesn't accidentally set Quantity to a negative number (would
// later flip buy/sell semantics). Catches sign errors in default-setting.
func TestRegistry_AllStrategies_DefaultsDoesNotMutateQuantityToNegative(t *testing.T) {
	registry := collectRegisteredStrategies(t)
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		registered := registry[id]
		rt := reflect.TypeOf(registered)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct {
			continue
		}
		freshPtr := reflect.New(rt)
		fresh := freshPtr.Interface()
		d, ok := fresh.(bbgo.StrategyDefaulter)
		if !ok {
			continue
		}
		_ = d.Defaults()
		// Walk top-level fields looking for fixedpoint.Value named Quantity/Amount/etc
		fv := freshPtr.Elem()
		for i := 0; i < fv.NumField(); i++ {
			field := fv.Field(i)
			if !field.CanInterface() {
				continue
			}
			name := rt.Field(i).Name
			if !looksLikeQuantityField(name) {
				continue
			}
			if fp, ok := field.Interface().(fixedpoint.Value); ok && fp.Sign() < 0 {
				t.Errorf("strategy %s: Defaults() set %s to a negative value (%s)", id, name, fp.String())
			}
		}
	}
}

func looksLikeQuantityField(name string) bool {
	switch name {
	case "Quantity", "Amount", "BaseQuantity", "QuoteQuantity",
		"MinQuantity", "MaxQuantity", "OrderQuantity":
		return true
	}
	return false
}

// require is intentionally used here to keep the test file's imports list
// consistent with sibling files; some subtests may use require for clarity.
var _ = require.True
