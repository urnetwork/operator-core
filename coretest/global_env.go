package coretest

import (
	"testing"

	operator "github.com/urnetwork/operator-core"
)

// Sets env as operator's global Env for one test, and puts back the Env that stood before it when the
// test ends: the one an outer helper set, or the refusing Env when none was. A nil env sets the refusing
// Env for the test, for code that must be seen to fail closed with none set.
//
// The global Env is one to a process, so a test that calls this must not run in parallel with another
// that reads or sets it.
func SetGlobalEnv(t testing.TB, env operator.Env) {
	t.Helper()
	previous := operator.GlobalEnv()
	operator.SetGlobalEnv(env)
	t.Cleanup(func() {
		operator.SetGlobalEnv(previous)
	})
}
