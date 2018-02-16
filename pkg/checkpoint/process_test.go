package checkpoint

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProcess(t *testing.T) {
	type testCase struct {
		desc                string
		localRunning        map[string]*v1.Pod
		localParents        map[string]*v1.Pod
		apiParents          map[string]*v1.Pod
		activeCheckpoints   map[string]*v1.Pod
		inactiveCheckpoints map[string]*v1.Pod
		expectStart         []string
		expectStop          []string
		expectRemove        []string
		expectGraceStart    []string
		expectGraceStop     []string
		expectGraceRemove   []string
		podName             string
	}

	cases := []testCase{
		{
			desc:                "Inactive checkpoint and no local running: should start",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			expectStart:         []string{"AA"},
		},
		{
			desc:                "Inactive checkpoint and local running: no change",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:                "Inactive checkpoint and no api parent: should remove",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"BB": {}},
			expectGraceRemove:   []string{"AA"},
		},
		{
			desc:                "Inactive checkpoint and both api & local running: no change",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:                "Inactive checkpoint and only api parent: should start",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"AA": {}},
			expectStart:         []string{"AA"},
		},
		{
			desc:                "Inactive checkpoint and only api and kubelet parents: should start",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"AA": {}},
			localParents:        map[string]*v1.Pod{"AA": {}},
			expectStart:         []string{"AA"},
		},
		{
			desc:              "Active checkpoint and no local running: no change",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint and local running: should stop",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:      map[string]*v1.Pod{"AA": {}},
			expectStop:        []string{"AA"},
		},
		{
			desc:              "Active checkpoint and api parent: no change",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint and no api parent: should remove",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"BB": {}},
			expectGraceRemove: []string{"AA"},
		},
		{
			desc:              "Active checkpoint with local running, and api parent: should stop",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:      map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"AA": {}},
			expectStop:        []string{"AA"},
		},
		{
			desc:              "Active checkpoint with local parent, and no api parent: should remove",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localParents:      map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"BB": {}},
			expectGraceRemove: []string{"AA"},
		},
		{
			desc:                "Both active and inactive checkpoints, with no api parent: remove both",
			activeCheckpoints:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"BB": {}},
			expectGraceRemove:   []string{"AA"}, // Only need single remove, we should clean up both active/inactive
		},
		{
			desc:                "Inactive checkpoint, local parent, local running, no api parent: no change", // Safety check - don't GC if local parent still exists (even if possibly stale)
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
			localParents:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint, local parent, no local running, no api parent: no change", // Safety check - don't GC if local parent still exists (even if possibly stale)
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localParents:      map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:         "Inactive pod-checkpointer, local parent, local running, api parent: should start",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			apiParents: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart:      []string{"kube-system/pod-checkpointer"},
			expectGraceStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc: "Inactive pod-checkpointer, local parent, no local running, api not reachable: should start",
			localParents: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart:      []string{"kube-system/pod-checkpointer"},
			expectGraceStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, no local parent, no api parent: should remove in the last",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}, "BB": {}},
			localParents: map[string]*v1.Pod{"BB": {}},
			apiParents:   map[string]*v1.Pod{"BB": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectStart:       []string{"kube-system/pod-checkpointer"},
			expectStop:        []string{"BB"},
			expectGraceRemove: []string{"AA", "BB", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, no local parent, no api parent: should remove all",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectStart:       []string{"kube-system/pod-checkpointer"},
			expectGraceRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Active pod-checkpointer, no local parent, no api parent: should remove all",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			activeCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectStart:       []string{"kube-system/pod-checkpointer"},
			expectStop:        []string{"AA"},
			expectGraceRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, local parent, local running, api parent: should start",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			apiParents: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart:      []string{"kube-system/pod-checkpointer"},
			expectGraceStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:    "Running as an on-disk checkpointer: Inactive pod-checkpointer, local parent, no local running, api not reachable: should start",
			podName: "pod-checkpointer-mynode",
			localParents: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart:      []string{"kube-system/pod-checkpointer"},
			expectGraceStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, no local parent, no api parent: should remove in the last",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"BB": {}},
			apiParents:   map[string]*v1.Pod{"BB": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectStart:       []string{"kube-system/pod-checkpointer", "BB"},
			expectGraceRemove: []string{"AA", "BB", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, no local parent, no api parent: should remove all",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectStart:       []string{"kube-system/pod-checkpointer"},
			expectGraceRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Active pod-checkpointer, no local parent, no api parent: should remove all",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			activeCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectStart:       []string{"kube-system/pod-checkpointer"},
			expectStop:        []string{"AA"},
			expectGraceRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
	}

	for _, tc := range cases {
		// Set up test state.
		cp := CheckpointerPod{
			NodeName:     "mynode",
			PodName:      "pod-checkpointer",
			PodNamespace: "kube-system",
		}
		if tc.podName != "" {
			cp.PodName = tc.podName
		}
		c := checkpoints{}

		// Run test now.
		now := time.Time{}
		c.update(tc.localRunning, tc.localParents, tc.apiParents, tc.activeCheckpoints, tc.inactiveCheckpoints, cp)
		gotStart, gotStop, gotRemove := c.process(now, tc.apiParents != nil, tc.localRunning, tc.localParents, tc.apiParents)

		// Advance past grace period and test again.
		now = now.Add(checkpointGracePeriod)
		c.update(tc.localRunning, tc.localParents, tc.apiParents, tc.activeCheckpoints, tc.inactiveCheckpoints, cp)
		gotGraceStart, gotGraceStop, gotGraceRemove := c.process(now, tc.apiParents != nil, tc.localRunning, tc.localParents, tc.apiParents)
		if !reflect.DeepEqual(tc.expectStart, gotStart) ||
			!reflect.DeepEqual(tc.expectStop, gotStop) ||
			!reflect.DeepEqual(tc.expectRemove, gotRemove) ||
			!reflect.DeepEqual(tc.expectGraceStart, gotGraceStart) ||
			!reflect.DeepEqual(tc.expectGraceStop, gotGraceStop) ||
			!reflect.DeepEqual(tc.expectGraceRemove, gotGraceRemove) {
			t.Errorf("For test: %s\nExpected start: %s Got: %s\nExpected stop: %s Got: %s\nExpected remove: %s Got: %s\nExpected grace period start: %s Got: %s\nExpected grace period stop: %s Got: %s\nExpected grace period remove: %s Got: %s\n", tc.desc, tc.expectStart, gotStart, tc.expectStop, gotStop, tc.expectRemove, gotRemove, tc.expectGraceStart, gotGraceStart, tc.expectGraceStop, gotGraceStop, tc.expectGraceRemove, gotGraceRemove)
		}
	}
}
