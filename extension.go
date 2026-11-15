// The extension contract (§4.5; owner decision 251): what an extension hands its host, its migrations and
// its engines' constructors over an Env, and the registry a host keeps of the extensions it runs, so that
// hosting another extension is one import and one registration. The host iterates the registry: it runs
// each extension's migrations, in registration order and each extension's own order, and builds an
// engine over the Env of the process that hosts it.

package operator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

var (
	// An extension or migration the registry refuses: no name, a name taken, a migration without an id or
	// with both or neither of Sql and Code, an id used twice.
	ErrExtensionInvalid = errors.New("extension invalid")
	// No registered extension builds an engine for the service, or two do.
	ErrEngineUnknown = errors.New("engine unknown")
)

// Builds one engine over the Env of the process that hosts it. An error leaves nothing running.
type EngineFunction func(ctx context.Context, env Env) (Engine, error)

// One schema step an extension owns. Exactly one of Sql and Code is set. A step the host has applied never
// changes or moves, and each is additive and works with the release running when it applies
// (connect/CODESTYLE.md's migration rules).
type Migration struct {
	// Stable and unique within the extension: the host records what has run by extension and id.
	Id string
	// Statements the host applies in one transaction over its own connection.
	Sql string
	// A data step the host runs over its db instead.
	Code func(ctx context.Context, db Db) error
}

// ErrExtensionInvalid for a step without an id, or with both or neither of Sql and Code.
func (self *Migration) Validate() error {
	if self == nil {
		return fmt.Errorf("%w: a nil migration", ErrExtensionInvalid)
	}
	if self.Id == "" {
		return fmt.Errorf("%w: a migration without an id", ErrExtensionInvalid)
	}
	if (self.Sql == "") == (self.Code == nil) {
		return fmt.Errorf("%w: migration %s needs exactly one of Sql and Code", ErrExtensionInvalid, self.Id)
	}
	return nil
}

// What an extension hands its host. Each method answers the same for the life of the process.
type Extension interface {
	// Stable and unique among a host's extensions: the key of its migrations in the host's records.
	Name() string
	// Its schema steps, in the order they apply.
	Migrations() []*Migration
	// Its engines' constructors, by the service name of the block that hosts each (§19.4), such as "evs".
	Engines() map[string]EngineFunction
}

// The extensions one host runs, in registration order. The host makes one, registers each extension at
// startup, and iterates it. Safe for concurrent use.
type ExtensionRegistry struct {
	stateLock  sync.Mutex
	extensions []Extension
}

// Adds extension, refusing, and adding nothing, when its name is empty or taken or its migrations do not
// validate or reuse an id.
func (self *ExtensionRegistry) Register(extension Extension) error {
	if extension == nil {
		return fmt.Errorf("%w: a nil extension", ErrExtensionInvalid)
	}
	name := extension.Name()
	if name == "" {
		return fmt.Errorf("%w: an extension without a name", ErrExtensionInvalid)
	}
	migrationIds := map[string]bool{}
	for _, migration := range extension.Migrations() {
		if err := migration.Validate(); err != nil {
			return fmt.Errorf("extension %s: %w", name, err)
		}
		if migrationIds[migration.Id] {
			return fmt.Errorf("%w: extension %s uses migration id %s twice", ErrExtensionInvalid, name, migration.Id)
		}
		migrationIds[migration.Id] = true
	}
	self.stateLock.Lock()
	defer self.stateLock.Unlock()
	for _, registered := range self.extensions {
		if registered.Name() == name {
			return fmt.Errorf("%w: extension %s is registered already", ErrExtensionInvalid, name)
		}
	}
	self.extensions = append(self.extensions, extension)
	return nil
}

// The registered extensions, in registration order. A copy.
func (self *ExtensionRegistry) Extensions() []Extension {
	self.stateLock.Lock()
	defer self.stateLock.Unlock()
	return slices.Clone(self.extensions)
}

// The constructor of the one registered engine for service, ErrEngineUnknown when no extension or more
// than one builds one.
func (self *ExtensionRegistry) Engine(service string) (EngineFunction, error) {
	var found EngineFunction
	foundCount := 0
	for _, extension := range self.Extensions() {
		if function, ok := extension.Engines()[service]; ok && function != nil {
			found = function
			foundCount += 1
		}
	}
	switch foundCount {
	case 0:
		return nil, fmt.Errorf("%w: no extension builds %s", ErrEngineUnknown, service)
	case 1:
		return found, nil
	}
	return nil, fmt.Errorf("%w: %d extensions build %s", ErrEngineUnknown, foundCount, service)
}
