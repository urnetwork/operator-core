// The process's Env (QD-1027): the package's one piece of process-wide state. A hosting process sets it
// once at its start, before it serves, and code a host calls with no Env in hand, such as sidecar/work
// and sidecar/handler, reads it and passes it on. Code that is given an Env or a ServiceHost keeps taking
// it, and never reads this one.
//
// Before any set, and after a set of nil, GlobalEnv answers an Env that refuses every service, so a
// caller keeps the accessor contract and never meets a nil Env.

package operator

import (
	"reflect"
	"sync/atomic"
)

// What every refusal of the Env that stands before any set gives as its reason.
const globalEnvNotSetReason = "no global env was set"

// The Env that stands while none is set: every accessor refuses, naming itself.
type notSetEnv struct{}

var _ Env = (*notSetEnv)(nil)

// One for the process, so a caller that compares what GlobalEnv answers sees one value while none is set.
var globalEnvNotSet = &notSetEnv{}

func (self *notSetEnv) refusal(service string) error {
	return &NotGrantedError{Service: service, Reason: globalEnvNotSetReason}
}

func (self *notSetEnv) Db() (Db, error) {
	return nil, self.refusal("Db")
}

func (self *notSetEnv) Redis() (Redis, error) {
	return nil, self.refusal("Redis")
}

func (self *notSetEnv) Blobs() (BlobStore, error) {
	return nil, self.refusal("Blobs")
}

func (self *notSetEnv) Authenticator() (Authenticator, error) {
	return nil, self.refusal("Authenticator")
}

func (self *notSetEnv) Quota() (Quota, error) {
	return nil, self.refusal("Quota")
}

func (self *notSetEnv) Metrics() (Metrics, error) {
	return nil, self.refusal("Metrics")
}

func (self *notSetEnv) Tasks() (Tasks, error) {
	return nil, self.refusal("Tasks")
}

func (self *notSetEnv) Push() (Push, error) {
	return nil, self.refusal("Push")
}

func (self *notSetEnv) ServiceClient() (ServiceClientSource, error) {
	return nil, self.refusal("ServiceClient")
}

func (self *notSetEnv) Clock() (Clock, error) {
	return nil, self.refusal("Clock")
}

func (self *notSetEnv) ClockCheck() (ClockCheck, error) {
	return nil, self.refusal("ClockCheck")
}

func (self *notSetEnv) Gates() (ServiceGates, error) {
	return nil, self.refusal("Gates")
}

func (self *notSetEnv) Budget() (WriteBudget, error) {
	return nil, self.refusal("Budget")
}

func (self *notSetEnv) Abuse() (AbuseGate, error) {
	return nil, self.refusal("Abuse")
}

// The Env set, boxed so an atomic pointer holds an interface value of any concrete type.
type globalEnvValue struct {
	env Env
}

// nil while no Env is set.
var globalEnv atomic.Pointer[globalEnvValue]

// Whether env is no Env a caller could use: nil, an interface holding a nil pointer, map, slice, func or
// channel, or the Env that stands while none is set.
func isNoGlobalEnv(env Env) bool {
	if env == nil || env == Env(globalEnvNotSet) {
		return true
	}
	reflected := reflect.ValueOf(env)
	switch reflected.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return reflected.IsNil()
	}
	return false
}

// Sets the process's Env, which GlobalEnv answers from then on. A hosting process calls it once at its
// start, before it serves; a later set replaces the earlier one for every later read. nil, a nil pointer
// in an Env, or the refusing Env that GlobalEnv answered before any set, puts that refusing Env back,
// which a test uses to restore. Safe for concurrent use.
func SetGlobalEnv(env Env) {
	if isNoGlobalEnv(env) {
		globalEnv.Store(nil)
		return
	}
	globalEnv.Store(&globalEnvValue{env: env})
}

// The process's Env: the one last set, or, while none is set, an Env whose every accessor answers a
// *NotGrantedError that wraps ErrNotGranted and whose Reason says no global env was set. Never nil. Safe
// for concurrent use, and it does no I/O, so a caller may read it per request rather than keep it.
func GlobalEnv() Env {
	if value := globalEnv.Load(); value != nil {
		return value.env
	}
	return globalEnvNotSet
}
