package checkpoint

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
)

// checkpoint holds the state of a single checkpointed pod. A checkpoint can move between states
// based on the apiCondition that the checkpointer sees.
type checkpoint struct {
	// name is the name of the checkpointed pod.
	name string
	// pod is the most up-to-date v1.Pod data.
	pod *v1.Pod
	// state is the current state of the checkpoint.
	state checkpointState
}

// String() implements fmt.Stringer.String().
func (c checkpoint) String() string {
	return fmt.Sprintf("%s (%s)", c.name, c.state)
}

// checkpoints holds the state of the checkpoints.
type checkpoints struct {
	checkpoints    map[string]*checkpoint
	selfCheckpoint *checkpoint
}

// update updates the checkpoints using the information retrieved from the various API endpoints.
func (cs *checkpoints) update(localRunningPods, localParentPods, apiParentPods, activeCheckpoints, inactiveCheckpoints map[string]*v1.Pod, checkpointerPod CheckpointerPod) {
	if cs.checkpoints == nil {
		cs.checkpoints = make(map[string]*checkpoint)
	}

	// Temporarily add the self-checkpointer into the map so it is updated as well.
	if cs.selfCheckpoint != nil {
		cs.checkpoints[cs.selfCheckpoint.name] = cs.selfCheckpoint
	}

	// Make sure all on-disk checkpoints are represented in memory, i.e. if we are restarting from a
	// crash.
	for name, pod := range activeCheckpoints {
		if _, ok := cs.checkpoints[name]; !ok {
			cs.checkpoints[name] = &checkpoint{
				name:  name,
				state: stateActive{},
				pod:   pod,
			}
			// Override the state for the self-checkpointer.
			if isPodCheckpointer(pod, checkpointerPod) {
				cs.checkpoints[name].state = stateSelfCheckpointActive{}
			}
		}
	}

	for name, pod := range inactiveCheckpoints {
		if _, ok := cs.checkpoints[name]; !ok {
			cs.checkpoints[name] = &checkpoint{
				name:  name,
				state: stateInactive{},
				pod:   pod,
			}
			// Override the state for the self-checkpointer.
			if isPodCheckpointer(pod, checkpointerPod) {
				cs.checkpoints[name].state = stateSelfCheckpointActive{}
			}
		}
	}

	// Add union of parent pods from other sources.
	for name, pod := range localParentPods {
		if _, ok := cs.checkpoints[name]; !ok {
			cs.checkpoints[name] = &checkpoint{
				name:  name,
				state: stateNone{},
			}
		}
		// Always overwrite with the localParentPod since it is a better source of truth.
		cs.checkpoints[name].pod = pod
	}

	for name, pod := range apiParentPods {
		if _, ok := cs.checkpoints[name]; !ok {
			cs.checkpoints[name] = &checkpoint{
				name:  name,
				state: stateNone{},
			}
		}
		// Always overwrite with the apiParentPod since it is a better source of truth.
		cs.checkpoints[name].pod = pod
	}

	// Find the self-checkpointer pod if it exists and remove it from the map.
	for _, cp := range cs.checkpoints {
		if isPodCheckpointer(cp.pod, checkpointerPod) {
			// Separate the self-checkpoint from the map, as it is handled separately.
			cs.selfCheckpoint = cp
			delete(cs.checkpoints, cp.name)

			// If this is a new self-checkpoint make sure the state is set correctly.
			if cp.state.action() == none {
				cp.state = stateSelfCheckpointActive{}
			}
		}
	}
}

// process uses the apiserver inputs and curren time to determine which checkpoints to start, stop,
// or remove.
func (cs *checkpoints) process(now time.Time, apiAvailable bool, localRunningPods, localParentPods, apiParentPods map[string]*v1.Pod) (starts, stops, removes []string) {
	// The checkpointer must be handled specially: the checkpoint always needs to remain active, and
	// if it is removed from the apiserver then all other checkpoints need to be removed first.
	if cs.selfCheckpoint != nil {
		state := cs.selfCheckpoint.state.transition(now, apiCondition{
			apiAvailable: apiAvailable,
			apiParent:    apiParentPods[cs.selfCheckpoint.name] != nil,
			localRunning: localRunningPods[cs.selfCheckpoint.name] != nil,
			localParent:  localParentPods[cs.selfCheckpoint.name] != nil,
		})

		if state != cs.selfCheckpoint.state {
			glog.Infof("Self-checkpoint %s transitioning from state %s -> state %s", cs.selfCheckpoint, cs.selfCheckpoint.state, state)
			cs.selfCheckpoint.state = state
		}

		switch cs.selfCheckpoint.state.action() {
		case none:
			glog.Errorf("Unexpected transition to state %s with action 'none'", state)
		case start:
			// The selfCheckpoint must always be active to ensure that it can perform its functions in
			// the face of a full control plane restart.
			starts = append(starts, cs.selfCheckpoint.name)
		case stop:
			// If the checkpointer is stopped then stop all checkpoints. Next cycle they may restart.
			for name, cp := range cs.checkpoints {
				if cp.state.action() != none {
					stops = append(stops, name)
				}
			}
			sort.Strings(stops) // Ensure stable ordering.
			stops = append(stops, cs.selfCheckpoint.name)
			return starts, stops, removes
		case remove:
			// If the checkpointer is removed then remove all checkpoints, putting the selfCheckpoint
			// last and removing all state.
			for name, cp := range cs.checkpoints {
				if cp.state.action() != none {
					removes = append(removes, name)
					delete(cs.checkpoints, name)
				}
			}
			sort.Strings(removes) // Ensure stable ordering.
			removes = append(removes, cs.selfCheckpoint.name)
			delete(cs.checkpoints, cs.selfCheckpoint.name)
			cs.selfCheckpoint = nil
			return starts, stops, removes
		default:
			panic(fmt.Sprintf("unhandled action: %s", cs.selfCheckpoint.state.action()))
		}
	}

	// Update states for all the checkpoints and compute which to start / stop / remove.
	for name, cp := range cs.checkpoints {
		state := cp.state.transition(now, apiCondition{
			apiAvailable: apiAvailable,
			apiParent:    apiParentPods[name] != nil,
			localRunning: localRunningPods[name] != nil,
			localParent:  localParentPods[name] != nil,
		})

		if state != cp.state {
			// Apply state transition.
			// TODO(diegs): always apply this.
			if cp.state.action() != state.action() {
				switch state.action() {
				case none:
					glog.Errorf("Unexpected transition to state %s with action 'none'", state)
				case start:
					starts = append(starts, cp.name)
				case stop:
					stops = append(stops, cp.name)
				case remove:
					removes = append(removes, cp.name)
					delete(cs.checkpoints, cp.name)
				default:
					panic(fmt.Sprintf("unhandled action: %s", state.action()))
				}
			}

			glog.Infof("Checkpoint %s transitioning from state %s -> state %s", cp, cp.state, state)
			cp.state = state
		}
	}

	return starts, stops, removes
}

// createCheckpointsForValidParents will iterate through pods which are candidates for checkpointing, then:
// - checkpoint any remote assets they need (e.g. secrets, configmaps)
// - sanitize their podSpec, removing unnecessary information
// - store the manifest on disk in an "inactive" checkpoint location
func (c *checkpointer) createCheckpointsForValidParents() {
	// Assemble the list of parent pods to checkpoint.
	var parents []*v1.Pod
	for _, cp := range c.checkpoints.checkpoints {
		parents = append(parents, cp.pod)
	}
	if c.checkpoints.selfCheckpoint != nil {
		parents = append(parents, c.checkpoints.selfCheckpoint.pod)
	}

	// Update the checkpoints.
	needsCheckpointUpdate := lastCheckpoint.IsZero() || time.Since(lastCheckpoint) >= defaultCheckpointTimeout

	for _, pod := range parents {
		id := podFullName(pod)

		cp, err := copyPod(pod)
		if err != nil {
			glog.Errorf("Failed to create checkpoint pod copy for %s: %v", id, err)
			continue
		}

		cp = sanitizeCheckpointPod(cp)

		podChanged, err := writeCheckpointManifest(cp)
		if err != nil {
			glog.Errorf("Failed to write checkpoint for %s: %v", id, err)
			continue
		}

		// Check for secret and configmap changes if the pods have change or they haven't been checked in a while
		if podChanged || needsCheckpointUpdate {

			_, err = c.checkpointSecretVolumes(pod)
			if err != nil {
				//TODO(aaron): This can end up spamming logs at times when api-server is unavailable. To reduce spam
				//             we could only log error if api-server can't be contacted and existing secret doesn't exist.
				glog.Errorf("Failed to checkpoint secrets for pod %s: %v", id, err)
				continue
			}

			_, err = c.checkpointConfigMapVolumes(pod)
			if err != nil {
				//TODO(aaron): This can end up spamming logs at times when api-server is unavailable. To reduce spam
				//             we could only log error if api-server can't be contacted and existing configmap doesn't exist.
				glog.Errorf("Failed to checkpoint configMaps for pod %s: %v", id, err)
				continue
			}
		}
	}

	// If the secrets/manifests were checked update the lastCheckpoint
	if needsCheckpointUpdate {
		lastCheckpoint = time.Now()
	}
}

func handleRemove(remove []string) {
	for _, id := range remove {
		glog.Infof("Removing checkpoint of: %s", id)

		// Remove Secrets
		p := podFullNameToSecretPath(id)
		if err := os.RemoveAll(p); err != nil {
			glog.Errorf("Failed to remove pod secrets from %s: %s", p, err)
		}

		// Remove ConfipMaps
		p = podFullNameToConfigMapPath(id)
		if err := os.RemoveAll(p); err != nil {
			glog.Errorf("Failed to remove pod configMaps from %s: %s", p, err)
		}

		// Remove inactive checkpoints
		p = podFullNameToInactiveCheckpointPath(id)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			glog.Errorf("Failed to remove inactive checkpoint %s: %v", p, err)
			continue
		}

		// Remove active checkpoints.
		// We do this as the last step because we want to clean up
		// resources before the checkpointer itself exits.
		//
		// TODO(yifan): Removing the pods after removing the secrets/configmaps
		// might disturb other pods since they might want to use the configmap
		// or secrets during termination.
		//
		// However, since we are not waiting for them to terminate anyway, so it's
		// ok to just leave as is for now. We can handle this more gracefully later.
		p = podFullNameToActiveCheckpointPath(id)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			glog.Errorf("Failed to remove active checkpoint %s: %v", p, err)
			continue
		}
	}
}

func handleStop(stop []string) {
	for _, id := range stop {
		glog.Infof("Stopping active checkpoint: %s", id)
		p := podFullNameToActiveCheckpointPath(id)
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) { // Sanity check (it's fine - just want to surface this if it's occurring)
				glog.Warningf("Attempted to remove active checkpoint, but manifest no longer exists: %s", p)
			} else {
				glog.Errorf("Failed to stop active checkpoint %s: %v", p, err)
			}
		}
	}
}

func handleStart(start []string) {
	for _, id := range start {
		src := podFullNameToInactiveCheckpointPath(id)
		data, err := ioutil.ReadFile(src)
		if err != nil {
			glog.Errorf("Failed to read checkpoint source: %v", err)
			continue
		}

		dst := podFullNameToActiveCheckpointPath(id)
		if _, err := writeManifestIfDifferent(dst, id, data); err != nil {
			glog.Errorf("Failed to write active checkpoint manifest: %v", err)
		}
	}
}
