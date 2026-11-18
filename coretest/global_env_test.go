package coretest

import (
	"testing"

	operator "github.com/urnetwork/operator-core"
)

// The Env that stands before any set keeps the Env contract with nothing granted.
func TestGlobalEnvKeepsTheContractBeforeAnySet(t *testing.T) {
	SetGlobalEnv(t, nil)
	EnvSuite(t, operator.GlobalEnv())
}

// The helper sets its Env for the test that called it, and each test's end puts back what stood before:
// the outer test's Env after the inner one, and the refusing Env after the outer one.
func TestSetGlobalEnvRestoresThePreviousEnv(t *testing.T) {
	operator.SetGlobalEnv(nil)
	unset := operator.GlobalEnv()
	outer := &clockOnlyEnv{clock: &stillClock{}}
	inner := &clockOnlyEnv{clock: &stillClock{}}
	// The subtest boundary is the point here: a helper's restore runs at the end of the test it was given.
	t.Run("outer", func(t *testing.T) {
		SetGlobalEnv(t, outer)
		if operator.GlobalEnv() != operator.Env(outer) {
			t.Fatalf("the helper did not set its Env")
		}
		EnvSuite(t, operator.GlobalEnv(), "Clock")
		t.Run("inner", func(t *testing.T) {
			SetGlobalEnv(t, inner)
			if operator.GlobalEnv() != operator.Env(inner) {
				t.Fatalf("the inner helper did not set its Env")
			}
		})
		if operator.GlobalEnv() != operator.Env(outer) {
			t.Errorf("the inner test's end did not put the outer Env back")
		}
	})
	if operator.GlobalEnv() != unset {
		t.Errorf("the outer test's end did not put the refusing Env back")
	}
}

// A nil Env sets the refusing Env for the test, over whatever stood before, and that comes back after.
func TestSetGlobalEnvNilRefusesForTheTest(t *testing.T) {
	standing := &clockOnlyEnv{clock: &stillClock{}}
	SetGlobalEnv(t, standing)
	// The subtest boundary is the point here too: the nil set's restore runs at the subtest's end.
	t.Run("none", func(t *testing.T) {
		SetGlobalEnv(t, nil)
		EnvSuite(t, operator.GlobalEnv())
	})
	if operator.GlobalEnv() != operator.Env(standing) {
		t.Errorf("the Env that stood before a nil set did not come back")
	}
}
