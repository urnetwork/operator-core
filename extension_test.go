package operator

import (
	"context"
	"errors"
	"testing"
)

// An extension of fixed parts.
type testExtension struct {
	name       string
	migrations []*Migration
	engines    map[string]EngineFunction
}

func (self *testExtension) Name() string {
	return self.name
}

func (self *testExtension) Migrations() []*Migration {
	return self.migrations
}

func (self *testExtension) Engines() map[string]EngineFunction {
	return self.engines
}

func testEngineFunction(ctx context.Context, env Env) (Engine, error) {
	return nil, errors.New("not built in this test")
}

func TestExtensionRegistryRefusesWhatItCannotKey(t *testing.T) {
	for _, refused := range []Extension{
		nil,
		&testExtension{name: ""},
		&testExtension{name: "a", migrations: []*Migration{nil}},
		&testExtension{name: "a", migrations: []*Migration{{Sql: "CREATE TABLE a ()"}}},
		&testExtension{name: "a", migrations: []*Migration{{Id: "a1"}}},
		&testExtension{name: "a", migrations: []*Migration{{Id: "a1", Sql: "CREATE TABLE a ()", Code: func(ctx context.Context, db Db) error { return nil }}}},
		&testExtension{name: "a", migrations: []*Migration{{Id: "a1", Sql: "CREATE TABLE a ()"}, {Id: "a1", Sql: "CREATE TABLE b ()"}}},
	} {
		var registry ExtensionRegistry
		if err := registry.Register(refused); !errors.Is(err, ErrExtensionInvalid) {
			t.Errorf("%+v: %v", refused, err)
		}
		if len(registry.Extensions()) != 0 {
			t.Errorf("%+v was added", refused)
		}
	}
}

func TestExtensionRegistryKeepsOrderAndRefusesATakenName(t *testing.T) {
	var registry ExtensionRegistry
	first := &testExtension{name: "sidecar", migrations: []*Migration{{Id: "mg0", Sql: "CREATE TABLE a ()"}}}
	second := &testExtension{name: "other"}
	if err := registry.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(second); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(&testExtension{name: "sidecar"}); !errors.Is(err, ErrExtensionInvalid) {
		t.Errorf("a taken name: %v", err)
	}
	extensions := registry.Extensions()
	if len(extensions) != 2 || extensions[0] != first || extensions[1] != second {
		t.Errorf("extensions %v", extensions)
	}
}

func TestExtensionRegistryFindsTheOneEngineForAService(t *testing.T) {
	var registry ExtensionRegistry
	if _, err := registry.Engine("evs"); !errors.Is(err, ErrEngineUnknown) {
		t.Errorf("an empty registry: %v", err)
	}
	if err := registry.Register(&testExtension{name: "a", engines: map[string]EngineFunction{"evs": testEngineFunction}}); err != nil {
		t.Fatal(err)
	}
	if function, err := registry.Engine("evs"); err != nil || function == nil {
		t.Errorf("evs: %v", err)
	}
	if _, err := registry.Engine("gkm"); !errors.Is(err, ErrEngineUnknown) {
		t.Errorf("a service no extension builds: %v", err)
	}
	if err := registry.Register(&testExtension{name: "b", engines: map[string]EngineFunction{"evs": testEngineFunction}}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Engine("evs"); !errors.Is(err, ErrEngineUnknown) {
		t.Errorf("a service two extensions build: %v", err)
	}
}
