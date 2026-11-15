// Package coretest holds operator-core's conformance suites (owner decision 251): what every
// implementation of the host contract must do, run by a host against its real services (server/core on
// the private test stack) and, from the build's review phase, by the in-memory fakes that move here from
// sidecartest. Only test legs import it.
//
// This is the suites' skeleton: EnvSuite and DbSuite hold the shapes every implementation shares. The
// per-service suites (Redis, the file clusters, the write budget and the rest) arrive with the fakes.
package coretest

import (
	"errors"
	"reflect"
	"testing"

	operator "github.com/urnetwork/operator-core"
)

// One Env accessor, by the name its NotGrantedError carries.
type envAccessor struct {
	name string
	get  func(env operator.Env) (any, error)
}

// Every accessor of operator.Env, in its declaration order.
func envAccessors() []envAccessor {
	return []envAccessor{
		{name: "Db", get: func(env operator.Env) (any, error) { return env.Db() }},
		{name: "Redis", get: func(env operator.Env) (any, error) { return env.Redis() }},
		{name: "Blobs", get: func(env operator.Env) (any, error) { return env.Blobs() }},
		{name: "Authenticator", get: func(env operator.Env) (any, error) { return env.Authenticator() }},
		{name: "Quota", get: func(env operator.Env) (any, error) { return env.Quota() }},
		{name: "Metrics", get: func(env operator.Env) (any, error) { return env.Metrics() }},
		{name: "Tasks", get: func(env operator.Env) (any, error) { return env.Tasks() }},
		{name: "Push", get: func(env operator.Env) (any, error) { return env.Push() }},
		{name: "ServiceClient", get: func(env operator.Env) (any, error) { return env.ServiceClient() }},
		{name: "Clock", get: func(env operator.Env) (any, error) { return env.Clock() }},
		{name: "ClockCheck", get: func(env operator.Env) (any, error) { return env.ClockCheck() }},
		{name: "Gates", get: func(env operator.Env) (any, error) { return env.Gates() }},
		{name: "Budget", get: func(env operator.Env) (any, error) { return env.Budget() }},
		{name: "Abuse", get: func(env operator.Env) (any, error) { return env.Abuse() }},
	}
}

// The names EnvSuite reads granted from, so a caller names only accessors that exist.
func EnvAccessorNames() []string {
	names := []string{}
	for _, accessor := range envAccessors() {
		names = append(names, accessor.name)
	}
	return names
}

// Whether value is nil, or an interface holding a nil pointer, map, slice, func or channel: a service no
// caller could use.
func isNilService(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return reflected.IsNil()
	}
	return false
}

// Holds env to the Env contract: each accessor granted names answers a usable service and no error, and
// every other answers no service and an error that wraps operator.ErrNotGranted, a *NotGrantedError
// naming that accessor. No accessor answers nil with a nil error. A name in granted that no accessor has
// fails, so a renamed accessor cannot pass unread.
func EnvSuite(t testing.TB, env operator.Env, granted ...string) {
	t.Helper()
	grantedNames := map[string]bool{}
	for _, name := range granted {
		grantedNames[name] = true
	}
	for _, accessor := range envAccessors() {
		service, err := accessor.get(env)
		if grantedNames[accessor.name] {
			if err != nil {
				t.Errorf("%s: granted, and answered %v", accessor.name, err)
			}
			if isNilService(service) {
				t.Errorf("%s: granted, and answered no service", accessor.name)
			}
			delete(grantedNames, accessor.name)
			continue
		}
		if !isNilService(service) {
			t.Errorf("%s: not granted, and answered a service", accessor.name)
		}
		if !errors.Is(err, operator.ErrNotGranted) {
			t.Errorf("%s: not granted, and answered %v, which is not ErrNotGranted", accessor.name, err)
			continue
		}
		var notGranted *operator.NotGrantedError
		if !errors.As(err, &notGranted) || notGranted.Service != accessor.name {
			t.Errorf("%s: the refusal %v names another service", accessor.name, err)
		}
	}
	for name := range grantedNames {
		t.Errorf("%s: granted, and operator.Env has no such accessor", name)
	}
}
