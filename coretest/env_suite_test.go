package coretest

import (
	"slices"
	"strings"
	"testing"
	"time"

	operator "github.com/urnetwork/operator-core"
)

// An Env that grants only a clock and refuses the rest, the shape of evs's grants for the db.
type clockOnlyEnv struct {
	// what a refusal returns, which a broken Env bends
	refuse func(service string) error
	clock  operator.Clock
}

func (self *clockOnlyEnv) refusal(service string) error {
	if self.refuse != nil {
		return self.refuse(service)
	}
	return &operator.NotGrantedError{Service: service, Reason: "this test grants a clock alone"}
}

func (self *clockOnlyEnv) Db() (operator.Db, error) { return nil, self.refusal("Db") }
func (self *clockOnlyEnv) Redis() (operator.Redis, error) {
	return nil, self.refusal("Redis")
}
func (self *clockOnlyEnv) Blobs() (operator.BlobStore, error) {
	return nil, self.refusal("Blobs")
}
func (self *clockOnlyEnv) Authenticator() (operator.Authenticator, error) {
	return nil, self.refusal("Authenticator")
}
func (self *clockOnlyEnv) Quota() (operator.Quota, error) {
	return nil, self.refusal("Quota")
}
func (self *clockOnlyEnv) Metrics() (operator.Metrics, error) {
	return nil, self.refusal("Metrics")
}
func (self *clockOnlyEnv) Tasks() (operator.Tasks, error) {
	return nil, self.refusal("Tasks")
}
func (self *clockOnlyEnv) Push() (operator.Push, error) { return nil, self.refusal("Push") }
func (self *clockOnlyEnv) ServiceClient() (operator.ServiceClientSource, error) {
	return nil, self.refusal("ServiceClient")
}
func (self *clockOnlyEnv) Clock() (operator.Clock, error) { return self.clock, nil }
func (self *clockOnlyEnv) ClockCheck() (operator.ClockCheck, error) {
	return nil, self.refusal("ClockCheck")
}
func (self *clockOnlyEnv) Gates() (operator.ServiceGates, error) {
	return nil, self.refusal("Gates")
}
func (self *clockOnlyEnv) Budget() (operator.WriteBudget, error) {
	return nil, self.refusal("Budget")
}
func (self *clockOnlyEnv) Abuse() (operator.AbuseGate, error) {
	return nil, self.refusal("Abuse")
}

// A clock that is never read here.
type stillClock struct{}

func (self *stillClock) Now() time.Time { return time.Time{} }
func (self *stillClock) After(timeout time.Duration) <-chan time.Time {
	return make(chan time.Time)
}
func (self *stillClock) AfterFunc(timeout time.Duration, function func()) func() bool {
	return func() bool { return true }
}

// A test the suite should fail, whose failures are collected instead of failing the caller.
type recordingTb struct {
	testing.TB
	failures []string
}

func (self *recordingTb) Errorf(format string, args ...any) {
	self.failures = append(self.failures, format)
}

func (self *recordingTb) Helper() {}

func TestEnvSuitePassesAnEnvThatKeepsTheContract(t *testing.T) {
	EnvSuite(t, &clockOnlyEnv{clock: &stillClock{}}, "Clock")
}

func TestEnvSuiteNamesEveryAccessor(t *testing.T) {
	names := EnvAccessorNames()
	if len(names) != 14 || !slices.Contains(names, "Db") || !slices.Contains(names, "Abuse") {
		t.Errorf("names %v", names)
	}
}

// The subtest boundary is the point here: each Env breaks the contract one way, and the suite must fail.
func TestEnvSuiteFailsEnvsThatBreakTheContract(t *testing.T) {
	for _, c := range []struct {
		name    string
		env     operator.Env
		granted []string
	}{
		{name: "a nil service with no error", env: &clockOnlyEnv{refuse: func(service string) error { return nil }, clock: &stillClock{}}, granted: []string{"Clock"}},
		{name: "a typed nil clock", env: &clockOnlyEnv{clock: (*stillClock)(nil)}, granted: []string{"Clock"}},
		{name: "a refusal naming another service", env: &clockOnlyEnv{refuse: func(service string) error {
			return &operator.NotGrantedError{Service: "Other"}
		}, clock: &stillClock{}}, granted: []string{"Clock"}},
		{name: "a granted name with no accessor", env: &clockOnlyEnv{clock: &stillClock{}}, granted: []string{"Clock", "Mail"}},
		{name: "a granted service refused", env: &clockOnlyEnv{clock: &stillClock{}}, granted: []string{"Clock", "Db"}},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			recorder := &recordingTb{TB: t}
			EnvSuite(recorder, c.env, c.granted...)
			if len(recorder.failures) == 0 {
				t.Errorf("%s passed the suite", c.name)
			}
		})
	}
}
