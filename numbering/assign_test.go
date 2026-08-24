package numbering

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingAllocator struct {
	keys []SequenceKey
	next int64
	err  error
}

func (a *recordingAllocator) Next(_ context.Context, key SequenceKey) (int64, error) {
	if a.err != nil {
		return 0, a.err
	}
	a.keys = append(a.keys, key)
	a.next++
	return a.next, nil
}

func consumerRegistry(t *testing.T) *Registry {
	t.Helper()
	registry, err := NewRegistry(map[string]DocTypeSpec{
		"TRANSPORT_TASK": {Prefix: "TT", Template: "TT-{yyyy}-{seq:4}"},
		"STATEMENT":      {Prefix: "STMT", Template: "STMT-{yyyy}{mm}-{seq:5}"},
		"LABEL":          {Prefix: "LB", Template: "LB{yyyy}{rand:4}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestNewRegistryValidates(t *testing.T) {
	if _, err := NewRegistry(map[string]DocTypeSpec{"": {Prefix: "X"}}); !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("empty doc type: %v", err)
	}
	if _, err := NewRegistry(map[string]DocTypeSpec{"A": {}}); !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("missing prefix and template: %v", err)
	}
	if _, err := NewRegistry(map[string]DocTypeSpec{"A": {Template: "{bogus}"}}); !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("unparsable template: %v", err)
	}
	if _, err := NewRegistry(map[string]DocTypeSpec{"A": {Prefix: "A", CheckAlgo: "crc"}}); !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("unknown check algo: %v", err)
	}
	if _, err := NewRegistry(map[string]DocTypeSpec{"A": {Prefix: "A", FiscalYearStartMonth: 13}}); !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("fiscal month out of range: %v", err)
	}
}

func TestDocTypeSpecDefaultTemplate(t *testing.T) {
	spec := DocTypeSpec{Prefix: "SHP"}
	if got := spec.EffectiveTemplate(); got != "SHP-{yyyy}{mm}-{seq:6}" {
		t.Fatalf("got %q", got)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 8, 24, 16, 47, 5, 0, time.UTC)
}

func TestAssignConfirmedConsumerCases(t *testing.T) {
	allocator := &recordingAllocator{}
	assigner, err := NewAssigner(consumerRegistry(t), allocator, WithNow(fixedNow))
	if err != nil {
		t.Fatal(err)
	}

	number, err := assigner.Assign(context.Background(), "tenant-1", "TRANSPORT_TASK")
	if err != nil {
		t.Fatal(err)
	}
	if number != "TT-2026-0001" {
		t.Fatalf("got %q, want TT-2026-0001", number)
	}
	if key := allocator.keys[0]; key != (SequenceKey{"tenant-1", "TRANSPORT_TASK", "2026"}) {
		t.Fatalf("yearly reset key: %+v", key)
	}

	number, err = assigner.Assign(context.Background(), "tenant-1", "STATEMENT")
	if err != nil {
		t.Fatal(err)
	}
	if number != "STMT-202608-00002" {
		t.Fatalf("got %q, want STMT-202608-00002", number)
	}
	if key := allocator.keys[1]; key.Period != "202608" {
		t.Fatalf("monthly reset key: %+v", key)
	}
}

func TestAssignSkipsAllocatorWithoutSequenceToken(t *testing.T) {
	allocator := &recordingAllocator{}
	assigner, err := NewAssigner(consumerRegistry(t), allocator, WithNow(fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assigner.Assign(context.Background(), "tenant-1", "LABEL"); err != nil {
		t.Fatal(err)
	}
	if len(allocator.keys) != 0 {
		t.Fatalf("no sequence token, allocator must not be called: %+v", allocator.keys)
	}
}

func TestAssignGuards(t *testing.T) {
	allocator := &recordingAllocator{}
	assigner, err := NewAssigner(consumerRegistry(t), allocator, WithNow(fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assigner.Assign(context.Background(), "", "TRANSPORT_TASK"); !errors.Is(err, ErrScopeRequired) {
		t.Fatalf("empty scope: %v", err)
	}
	if _, err := assigner.Assign(context.Background(), "tenant-1", "NOPE"); !errors.Is(err, ErrUnknownDocType) {
		t.Fatalf("unknown doc type: %v", err)
	}

	failing := &recordingAllocator{err: errors.New("boom")}
	assigner, err = NewAssigner(consumerRegistry(t), failing, WithNow(fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assigner.Assign(context.Background(), "tenant-1", "TRANSPORT_TASK"); err == nil {
		t.Fatal("allocator failure must propagate")
	}
}

func TestNewAssignerRequiresDependencies(t *testing.T) {
	if _, err := NewAssigner(nil, &recordingAllocator{}); err == nil {
		t.Fatal("nil registry must fail")
	}
	if _, err := NewAssigner(consumerRegistry(t), nil); err == nil {
		t.Fatal("nil allocator must fail")
	}
}

func TestAssignScrambledSeriesAreScopeIsolated(t *testing.T) {
	registry, err := NewRegistry(map[string]DocTypeSpec{
		"PKG": {Prefix: "PKG", Template: "PKG{seqx:6}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 两个 scope 各自从 1 开始；同序号在不同 scope 下应解相关。
	differs := false
	for i := 0; i < 20 && !differs; i++ {
		a := mustAssign(t, registry, "tenant-a", int64(i+1))
		b := mustAssign(t, registry, "tenant-b", int64(i+1))
		differs = a != b
	}
	if !differs {
		t.Fatal("scramble series must be isolated per scope")
	}
}

type fixedAllocator struct{ value int64 }

func (a fixedAllocator) Next(context.Context, SequenceKey) (int64, error) { return a.value, nil }

func mustAssign(t *testing.T, registry *Registry, scope string, seq int64) string {
	t.Helper()
	assigner, err := NewAssigner(
		registry, fixedAllocator{value: seq},
		WithNow(fixedNow), WithScrambleSecret("s3cr3t"),
	)
	if err != nil {
		t.Fatal(err)
	}
	number, err := assigner.Assign(context.Background(), scope, "PKG")
	if err != nil {
		t.Fatal(err)
	}
	return number
}
