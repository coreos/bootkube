package checkpoint

import (
	"fmt"
	"time"

	"github.com/golang/glog"
)

// apiCondition represents information returned from the various api endpoints for a given pod.
type apiCondition struct {
	// apiAvailable is true if the apiserver was reachable.
	apiAvailable bool
	// apiParent is true if the api parent pod exists.
	apiParent bool
	// localRunning is true if the CRI shim reports that the pod is running locally.
	localRunning bool
	// localParent is true if the kubelet parent pod exists.
	localParent bool
}

// String() implements fmt.Stringer.String().
func (a apiCondition) String() string {
	return fmt.Sprintf("apiAvailable=%t, apiParent=%t, localRunning=%t, localParent=%t", a.apiAvailable, a.apiParent, a.localRunning, a.localParent)
}

// action represents the action to be taken based on the state of a checkpoint.
type action int

const (
	// none is the default action of "do nothing".
	none = iota
	// start means that the checkpoint should be started.
	start
	// stop means that the checkpoint should be stopped.
	stop
	// remove means that the checkpoint should be garbage collected.
	remove
)

// String() implements fmt.Stringer.String().
func (a action) String() string {
	switch a {
	case none:
		return "none"
	case start:
		return "start"
	case stop:
		return "stop"
	case remove:
		return "remove"
	default:
		return "[unknown action]"
	}
}

// checkpointState represents the current state of a checkpoint.
type checkpointState interface {
	// transition computes the new state for the current time and information from various apis.
	transition(time.Time, apiCondition) checkpointState
	// action returns the action that should be taken for this state.
	action() action
}

// stateSelfCheckpointActive represents a checkpoint of the checkpointer itself, which has special
// behavior.
//
// stateSelfCheckpointActive can transition to stateActiveGracePeriod.
type stateSelfCheckpointActive struct{}

// transition implements state.transition()
func (s stateSelfCheckpointActive) transition(now time.Time, apis apiCondition) checkpointState {
	if !apis.apiAvailable {
		// If the apiserver is unavailable always stay in the selfCheckpoint state.
		return s
	}

	if apis.apiParent {
		// If the parent pod exists always stay in the selfCheckpoint state.
		return s
	}

	// The apiserver parent pod is deleted, transition to stateActiveGracePeriod.
	// TODO(diegs): this is a little hacky, perhaps clean it up with a constructor.
	return stateActiveGracePeriod{gracePeriodEnd: now.Add(checkpointGracePeriod)}.checkGracePeriod(now, apis)
}

// action implements state.action()
func (s stateSelfCheckpointActive) action() action {
	// The self-checkpoint should always be started.
	return start
}

// String() implements fmt.Stringer.String().
func (s stateSelfCheckpointActive) String() string {
	return "self-checkpoint"
}

// stateNone represents a new pod that has not been processed yet, so it has no checkpoint state.
//
// stateNone can transition to stateInactive, stateInactiveGracePeriod, or stateActive.
type stateNone struct{}

// transition implements state.transition()
func (s stateNone) transition(now time.Time, apis apiCondition) checkpointState {
	// Newly discovered pods are treated as mostly inactive, but only if there is either a local
	// running pod or kubelet parent pod. In other words, if the new pod is only reflected in the
	// apiserver we do not checkpoint it yet.
	if apis.localRunning || apis.localParent {
		return stateInactive{}.transition(now, apis)
	}
	return s
}

// action implements state.action()
func (s stateNone) action() action {
	return none
}

// String() implements fmt.Stringer.String().
func (s stateNone) String() string {
	return "none"
}

// stateInactive is a checkpoint that is currently sitting inactive on disk.
//
// stateInactive can transition to stateActive or stateInactiveGracePeriod.
type stateInactive struct{}

// transition implements state.transition()
func (s stateInactive) transition(now time.Time, apis apiCondition) checkpointState {
	if !apis.apiAvailable {
		// The apiserver is unavailable but the local copy is running, remain in stateInactive.
		if apis.localRunning {
			return s
		}

		// The apiserver is unavailable and the local pod is not running, transition to stateActive.
		return stateActive{}
	}

	if apis.apiParent {
		// The parent pod exists and the kubelet is running it, remain in stateInactive.
		if apis.localRunning {
			return s
		}

		// The parent pod exists but the kubelet is not running it, transition to stateActive.
		return stateActive{}
	}

	// The apiserver parent pod is deleted, transition to stateInactiveGracePeriod.
	// TODO(diegs): this is a little hacky, perhaps clean it up with a constructor.
	return stateInactiveGracePeriod{gracePeriodEnd: now.Add(checkpointGracePeriod)}.checkGracePeriod(now, apis)
}

// action implements state.action()
func (s stateInactive) action() action {
	return stop
}

// String() implements fmt.Stringer.String().
func (s stateInactive) String() string {
	return "inactive"
}

// stateInactiveGracePeriod is a checkpoint that is inactive but will be garbage collected after a
// grace period.
//
// stateInactiveGracePeriod can transition to stateInactive, stateActive, or stateRemove.
type stateInactiveGracePeriod struct {
	// gracePeriodEnd is the time when the grace period for this checkpoint is over and it should be
	// garbage collected.
	gracePeriodEnd time.Time
}

// transition implements state.transition()
func (s stateInactiveGracePeriod) transition(now time.Time, apis apiCondition) checkpointState {
	if !apis.apiAvailable {
		// The apiserver is unavailable but the local copy is running, remain in
		// stateInactiveGracePeriod.
		if apis.localRunning {
			return s.checkGracePeriod(now, apis)
		}

		// The apiserver is unavailable and the local pod is not running, transition to stateActive.
		return stateActive{}
	}

	if apis.apiParent {
		// The parent pod exists and the kubelet is running it, remain in inactive.
		if apis.localRunning {
			return stateInactive{}
		}

		// The parent pod exists but the kubelet is not running it, transition to stateActive.
		return stateActive{}
	}

	// The apiserver pod is still deleted, remain in stateInactiveGracePeriod.
	return s.checkGracePeriod(now, apis)
}

func (s stateInactiveGracePeriod) checkGracePeriod(now time.Time, apis apiCondition) checkpointState {
	// Override state to remove if the grace period has passed.
	if now.Equal(s.gracePeriodEnd) || now.After(s.gracePeriodEnd) {
		glog.Infof("Grace period exceeded for state %s", s)
		return stateRemove{}
	}
	return s
}

// action implements state.action()
func (s stateInactiveGracePeriod) action() action {
	return stop
}

// String() implements fmt.Stringer.String().
func (s stateInactiveGracePeriod) String() string {
	return "inactive (grace period)"
}

// stateActive is a checkpoint that is currently activated.
//
// stateActive can transition to stateInactive or stateActiveGracePeriod.
type stateActive struct{}

// transition implements state.transition()
func (s stateActive) transition(now time.Time, apis apiCondition) checkpointState {
	if !apis.apiAvailable {
		// The apiserver is unavailable but the local copy is running, transition to inactive.
		if apis.localRunning {
			return stateInactive{}
		}

		// The apiserver is unavailable and the local pod is not running, remain in stateActive.
		return s
	}

	if apis.apiParent {
		// The parent pod exists and the kubelet is running it, transition to inactive.
		if apis.localRunning {
			return stateInactive{}
		}

		// The parent pod exists but the kubelet is not running it, remain in stateActive.
		return s
	}

	// The apiserver pod is deleted, transition to stateActiveGracePeriod.
	// TODO(diegs): this is a little hacky, perhaps clean it up with a constructor.
	return stateActiveGracePeriod{gracePeriodEnd: now.Add(checkpointGracePeriod)}.checkGracePeriod(now, apis)
}

// action implements state.action()
func (s stateActive) action() action {
	return start
}

// String() implements fmt.Stringer.String().
func (s stateActive) String() string {
	return "active"
}

// stateActiveGracePeriod is a checkpoint that is active but will be garbage collected after a grace
// period.
//
// stateActiveGracePeriod can transition to stateActive, stateInactive, or stateInactiveGracePeriod.
type stateActiveGracePeriod struct {
	// gracePeriodEnd is the time when the grace period for this checkpoint is over and it should be
	// garbage collected.
	gracePeriodEnd time.Time
}

// transition implements state.transition()
func (s stateActiveGracePeriod) transition(now time.Time, apis apiCondition) checkpointState {
	if !apis.apiAvailable {
		// The apiserver is unavailable but the local copy is running, transition to stateInactive.
		if apis.localRunning {
			return stateInactive{}
		}

		// The apiserver is unavailable and the local pod is not running, remain in
		// stateActiveGracePeriod.
		return s.checkGracePeriod(now, apis)
	}

	if apis.apiParent {
		// The parent pod exists and the kubelet is running it, transition to stateInactive.
		if apis.localRunning {
			return stateInactive{}
		}

		// The parent pod exists but the kubelet is not running it, transition to stateActive.
		return stateActive{}
	}

	// The parent pod is deleted and the apiserver is back. Move to inactive grace period.
	return stateInactiveGracePeriod{gracePeriodEnd: now.Add(checkpointGracePeriod)}.checkGracePeriod(now, apis)
}

func (s stateActiveGracePeriod) checkGracePeriod(now time.Time, apis apiCondition) checkpointState {
	// Override state to stateInactiveGracePeriod.transition() as if the grace period has passed. This
	// has the effect of either transitioning to stateInactive or stateRemove.
	if now.Equal(s.gracePeriodEnd) || now.After(s.gracePeriodEnd) {
		glog.Infof("Grace period exceeded for state %s", s)
		return stateInactiveGracePeriod{gracePeriodEnd: now}.transition(now, apis)
	}
	return s
}

// action implements state.action()
func (s stateActiveGracePeriod) action() action {
	return start
}

// String() implements fmt.Stringer.String().
func (s stateActiveGracePeriod) String() string {
	return "active (grace period)"
}

// stateRemove is a checkpoint that is being garbage collected.
//
// It is a terminal state that can never transition to other states; checkpoints in this state are
// removed as part of the update loop.
type stateRemove struct{}

// transition implements state.transition()
func (s stateRemove) transition(now time.Time, apis apiCondition) checkpointState {
	// Remove is a terminal state. This should never actually be called.
	glog.Errorf("Unexpected call to transition() for state %s", s)
	return s
}

// action implements state.action()
func (s stateRemove) action() action {
	return remove
}

// String() implements fmt.Stringer.String().
func (s stateRemove) String() string {
	return "remove"
}
