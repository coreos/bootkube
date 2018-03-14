package checkpoint

import (
	"reflect"
	"testing"
	"time"
)

var (
	allAPIConditions []apiCondition
)

func init() {
	checkpointGracePeriod = time.Second

	bools := []bool{true, false}
	for _, apiAvailable := range bools {
		for _, apiParent := range bools {
			for _, localRunning := range bools {
				for _, localParent := range bools {
					allAPIConditions = append(allAPIConditions, apiCondition{
						apiAvailable, apiParent, localRunning, localParent,
					})
				}
			}
		}
	}
}

func TestAllowedStateTransitions(t *testing.T) {
	for _, tc := range []struct {
		state checkpointState
		want  []checkpointState
	}{{
		state: stateSelfCheckpointActive{},
		want:  []checkpointState{stateSelfCheckpointActive{}, stateActiveGracePeriod{}},
	}, {
		state: stateNone{},
		want:  []checkpointState{stateNone{}, stateInactive{}, stateInactiveGracePeriod{}, stateActive{}},
	}, {
		state: stateInactive{},
		want:  []checkpointState{stateInactive{}, stateInactiveGracePeriod{}, stateActive{}},
	}, {
		state: stateInactiveGracePeriod{},
		want:  []checkpointState{stateInactive{}, stateInactiveGracePeriod{}, stateActive{}, stateRemove{}},
	}, {
		state: stateActive{},
		want:  []checkpointState{stateActive{}, stateActiveGracePeriod{}, stateInactive{}},
	}, {
		state: stateActiveGracePeriod{},
		want:  []checkpointState{stateActiveGracePeriod{}, stateActive{}, stateInactive{}, stateInactiveGracePeriod{}, stateRemove{}},
	}, {
		state: stateRemove{},
		want:  []checkpointState{stateRemove{}},
	}} {
		for _, apis := range allAPIConditions {
			now := time.Time{}
			got := tc.state.transition(time.Time{}, apis)
			allowed := false
			for _, want := range tc.want {
				if reflect.TypeOf(got) == reflect.TypeOf(want) {
					allowed = true
					break
				}
			}
			if !allowed {
				t.Errorf("%s.transition(%s, %s) = %s, want: %s", tc.state, now.Format("04:05"), apis, got, tc.want)
			}
		}
	}
}
