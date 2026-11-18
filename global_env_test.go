package operator

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

// A clock that is never read here.
type testGlobalClock struct{}

func (self *testGlobalClock) Now() time.Time {
	return time.Time{}
}

func (self *testGlobalClock) After(timeout time.Duration) <-chan time.Time {
	return make(chan time.Time)
}

func (self *testGlobalClock) AfterFunc(timeout time.Duration, function func()) func() bool {
	return func() bool { return true }
}

// An Env that grants its clock and refuses the rest under its own reason, so a test tells which Env
// answered.
type testGlobalEnv struct {
	reason string
	clock  Clock
}

func newTestGlobalEnv(reason string) *testGlobalEnv {
	return &testGlobalEnv{reason: reason, clock: &testGlobalClock{}}
}

func (self *testGlobalEnv) refusal(service string) error {
	return &NotGrantedError{Service: service, Reason: self.reason}
}

func (self *testGlobalEnv) Db() (Db, error)       { return nil, self.refusal("Db") }
func (self *testGlobalEnv) Redis() (Redis, error) { return nil, self.refusal("Redis") }
func (self *testGlobalEnv) Blobs() (BlobStore, error) {
	return nil, self.refusal("Blobs")
}
func (self *testGlobalEnv) Authenticator() (Authenticator, error) {
	return nil, self.refusal("Authenticator")
}
func (self *testGlobalEnv) Quota() (Quota, error)     { return nil, self.refusal("Quota") }
func (self *testGlobalEnv) Metrics() (Metrics, error) { return nil, self.refusal("Metrics") }
func (self *testGlobalEnv) Tasks() (Tasks, error)     { return nil, self.refusal("Tasks") }
func (self *testGlobalEnv) Push() (Push, error)       { return nil, self.refusal("Push") }
func (self *testGlobalEnv) ServiceClient() (ServiceClientSource, error) {
	return nil, self.refusal("ServiceClient")
}
func (self *testGlobalEnv) Clock() (Clock, error) { return self.clock, nil }
func (self *testGlobalEnv) ClockCheck() (ClockCheck, error) {
	return nil, self.refusal("ClockCheck")
}
func (self *testGlobalEnv) Gates() (ServiceGates, error) {
	return nil, self.refusal("Gates")
}
func (self *testGlobalEnv) Budget() (WriteBudget, error) {
	return nil, self.refusal("Budget")
}
func (self *testGlobalEnv) Abuse() (AbuseGate, error) { return nil, self.refusal("Abuse") }

// Leaves the process with no Env set once the test ends, as it stood before.
func resetGlobalEnv(t *testing.T) {
	t.Helper()
	SetGlobalEnv(nil)
	t.Cleanup(func() {
		SetGlobalEnv(nil)
	})
}

// Every accessor Env declares, read off the interface so one added later is held to the same contract,
// called on env: its name, the service it answered and its error.
func callEveryAccessor(t *testing.T, env Env) map[string][2]any {
	t.Helper()
	answers := map[string][2]any{}
	envType := reflect.TypeOf((*Env)(nil)).Elem()
	envValue := reflect.ValueOf(env)
	for i := 0; i < envType.NumMethod(); i += 1 {
		name := envType.Method(i).Name
		results := envValue.MethodByName(name).Call(nil)
		if len(results) != 2 {
			t.Fatalf("%s: %d results, and an accessor answers a service and an error", name, len(results))
		}
		var service any
		if !results[0].IsNil() {
			service = results[0].Interface()
		}
		var err any
		if !results[1].IsNil() {
			err = results[1].Interface()
		}
		answers[name] = [2]any{service, err}
	}
	if len(answers) == 0 {
		t.Fatalf("Env declares no accessor")
	}
	return answers
}

// Holds every accessor of env to a refusal that names it, wraps ErrNotGranted and gives reason.
func requireEveryAccessorRefuses(t *testing.T, env Env, reason string) {
	t.Helper()
	for name, answer := range callEveryAccessor(t, env) {
		if answer[0] != nil {
			t.Errorf("%s: answered a service", name)
		}
		err, _ := answer[1].(error)
		if !errors.Is(err, ErrNotGranted) {
			t.Errorf("%s: answered %v, which is not ErrNotGranted", name, err)
			continue
		}
		var notGranted *NotGrantedError
		if !errors.As(err, &notGranted) {
			t.Errorf("%s: the refusal %v is no *NotGrantedError", name, err)
			continue
		}
		if notGranted.Service != name {
			t.Errorf("%s: the refusal names %q", name, notGranted.Service)
		}
		if notGranted.Reason != reason {
			t.Errorf("%s: the refusal's reason is %q, want %q", name, notGranted.Reason, reason)
		}
	}
}

func TestGlobalEnvRefusesEveryServiceBeforeAnySet(t *testing.T) {
	resetGlobalEnv(t)
	env := GlobalEnv()
	if env == nil {
		t.Fatalf("no Env before any set")
	}
	requireEveryAccessorRefuses(t, env, "no global env was set")
	if again := GlobalEnv(); again != env {
		t.Errorf("two reads with none set answered two Envs")
	}
}

func TestGlobalEnvAnswersTheEnvLastSet(t *testing.T) {
	resetGlobalEnv(t)
	first := newTestGlobalEnv("first")
	second := newTestGlobalEnv("second")
	SetGlobalEnv(first)
	if GlobalEnv() != Env(first) {
		t.Fatalf("the Env set is not the Env answered")
	}
	clock, err := GlobalEnv().Clock()
	if err != nil || clock != first.clock {
		t.Errorf("the Env set did not answer its own clock: %v", err)
	}
	if _, err := GlobalEnv().Db(); !errors.Is(err, ErrNotGranted) {
		t.Errorf("the Env set answered a db it refuses: %v", err)
	}
	SetGlobalEnv(second)
	if GlobalEnv() != Env(second) {
		t.Errorf("a later set did not replace the earlier one")
	}
}

func TestSetGlobalEnvNilPutsTheRefusingEnvBack(t *testing.T) {
	resetGlobalEnv(t)
	unset := GlobalEnv()
	var nilPointer *testGlobalEnv
	for name, none := range map[string]Env{
		"nil":               nil,
		"a nil pointer":     nilPointer,
		"the refusing Env":  unset,
		"the refusing type": (*notSetEnv)(nil),
	} {
		SetGlobalEnv(newTestGlobalEnv("set"))
		if GlobalEnv() == unset {
			t.Fatalf("%s: a set left the refusing Env in place", name)
		}
		SetGlobalEnv(none)
		if GlobalEnv() != unset {
			t.Errorf("%s: did not put the refusing Env back", name)
		}
		requireEveryAccessorRefuses(t, GlobalEnv(), "no global env was set")
	}
}

// Readers and setters run together, each reader holding every read to one of the Envs a setter sets or
// to the refusing one, and to answering as that Env does. Under -race this is the check that a read
// never sees a half-made value.
func TestGlobalEnvConcurrentReadsAndSets(t *testing.T) {
	resetGlobalEnv(t)
	unset := GlobalEnv()
	envs := []*testGlobalEnv{newTestGlobalEnv("a"), newTestGlobalEnv("b"), newTestGlobalEnv("c")}
	known := map[Env]*testGlobalEnv{}
	for _, env := range envs {
		known[env] = env
	}
	const roundCount = 2000
	start := make(chan struct{})
	failures := make(chan string, 16)
	fail := func(message string) {
		select {
		case failures <- message:
		default:
		}
	}
	var group sync.WaitGroup
	for setter := 0; setter < 4; setter += 1 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			for round := 0; round < roundCount; round += 1 {
				switch choice := (setter + round) % (len(envs) + 1); choice {
				case len(envs):
					SetGlobalEnv(nil)
				default:
					SetGlobalEnv(envs[choice])
				}
			}
		}()
	}
	for reader := 0; reader < 8; reader += 1 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			for round := 0; round < roundCount; round += 1 {
				env := GlobalEnv()
				if env == nil {
					fail("a read answered no Env")
					return
				}
				clock, err := env.Clock()
				if env == unset {
					if clock != nil || !errors.Is(err, ErrNotGranted) {
						fail("the refusing Env answered a clock")
						return
					}
					continue
				}
				set, ok := known[env]
				if !ok {
					fail("a read answered an Env nobody set")
					return
				}
				if err != nil || clock != set.clock {
					fail("a read answered an Env that is not whole")
					return
				}
			}
		}()
	}
	close(start)
	group.Wait()
	close(failures)
	for message := range failures {
		t.Error(message)
	}
}
