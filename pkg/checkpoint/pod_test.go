package checkpoint

import (
	"fmt"
	"testing"

	"k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSanitizeCheckpointPod(t *testing.T) {
	type testCase struct {
		desc     string
		pod      *v1.Pod
		expected *v1.Pod
	}
	trueVar := true

	cases := []testCase{
		{
			desc: "Pod name and namespace are preserved, checkpoint annotation added, owner points to parent",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Existing annotations are removed, checkpoint annotation added, owner points to parent",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "podname",
					Namespace:   "podnamespace",
					Annotations: map[string]string{"foo": "bar"},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Pod status is reset",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
				},
				Status: v1.PodStatus{Phase: "Pending"},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Service acounts are cleared",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
				},
				Spec: v1.PodSpec{ServiceAccountName: "foo", DeprecatedServiceAccount: "bar"},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Labels are preserved",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
					Labels:    map[string]string{"foo": "bar"},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Labels:          map[string]string{"foo": "bar"},
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "OwnerReference of checkpoint points to parent pod",
			pod: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
					UID:       "pod-uid",
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "v1", Kind: "Daemonset", Name: "daemonname", UID: "daemon-uid"},
					},
				},
			},
			expected: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "podname",
					Namespace:   "podnamespace",
					Annotations: map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "v1", Kind: "Pod", Name: "podname", UID: "pod-uid", Controller: &trueVar},
					},
				},
			},
		},
		{
			desc: "Pod is already sanitized.",
			pod: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "podname",
					Namespace:   "podnamespace",
					Annotations: map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "v1", Kind: "Pod", Name: "podname", UID: "pod-uid", Controller: &trueVar},
					},
				},
			},
			expected: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "podname",
					Namespace:   "podnamespace",
					Annotations: map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "v1", Kind: "Pod", Name: "podname", UID: "pod-uid", Controller: &trueVar},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		got := sanitizeCheckpointPod(tc.pod)
		if !apiequality.Semantic.DeepEqual(tc.expected, got) {
			t.Errorf("\nFor Test: %s\n\nExpected:\n%#v\nGot:\n%#v\n", tc.desc, tc.expected, got)
		}
	}
}
func TestPodListToParentPods(t *testing.T) {
	parentAPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "A", Namespace: "A", Annotations: map[string]string{shouldCheckpointAnnotation: "true"}}}
	parentBPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "B", Namespace: "B", Annotations: map[string]string{shouldCheckpointAnnotation: "true"}}}
	checkpointPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "C", Namespace: "C", Annotations: map[string]string{checkpointParentAnnotation: "foo/bar"}}}
	regularPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "D", Namespace: "D", Annotations: map[string]string{"meta": "data"}}}

	type testCase struct {
		desc     string
		podList  *v1.PodList
		expected map[string]*v1.Pod
	}

	cases := []testCase{
		{
			desc:     "Both regular pods, none are parents",
			podList:  &v1.PodList{Items: []v1.Pod{regularPod, regularPod}},
			expected: nil,
		},
		{
			desc:     "Regular and checkpoint pods, none are parents",
			podList:  &v1.PodList{Items: []v1.Pod{regularPod, checkpointPod}},
			expected: nil,
		},
		{
			desc:     "One parent and one regular pod: Should return only parent",
			podList:  &v1.PodList{Items: []v1.Pod{parentAPod, regularPod}},
			expected: map[string]*v1.Pod{"A/A": {}},
		},
		{
			desc:     "Two parent pods, should return both",
			podList:  &v1.PodList{Items: []v1.Pod{parentAPod, parentBPod}},
			expected: map[string]*v1.Pod{"A/A": {}, "B/B": {}},
		},
	}

	for _, tc := range cases {
		got := podListToParentPods(tc.podList)
		if len(got) != len(tc.expected) {
			t.Errorf("Expected: %d pods but got %d for test: %s", len(tc.expected), len(got), tc.desc)
		}
		for e := range tc.expected {
			if _, ok := got[e]; !ok {
				t.Errorf("Missing expected podFullName %s", e)
			}
		}
	}
}

func podWithAnnotations(a map[string]string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "podname",
			Namespace:   "podnamespace",
			Annotations: a,
		},
	}
}

func TestIsValidParent(t *testing.T) {
	type testCase struct {
		desc     string
		pod      *v1.Pod
		expected bool
	}

	cases := []testCase{
		{
			desc:     "Checkpoint pod",
			pod:      podWithAnnotations(map[string]string{checkpointParentAnnotation: "foo/bar"}),
			expected: false,
		},
		{
			desc:     "Static pod",
			pod:      podWithAnnotations(map[string]string{podSourceAnnotation: "file"}),
			expected: false,
		},
		{
			desc:     "Normal pod",
			pod:      podWithAnnotations(map[string]string{"foo": "bar"}),
			expected: false,
		},
		{
			desc:     "Parent pod",
			pod:      podWithAnnotations(map[string]string{shouldCheckpointAnnotation: "true"}),
			expected: true,
		},
		{
			desc:     "No annotation / normal pod",
			pod:      podWithAnnotations(nil),
			expected: false,
		},
		{
			desc: "Parent and static pod",
			pod: podWithAnnotations(map[string]string{
				shouldCheckpointAnnotation: "true",
				podSourceAnnotation:        "file",
			}),
			expected: false,
		},
		{
			desc: "Parent and checkpoint", // This should never happen
			pod: podWithAnnotations(map[string]string{
				shouldCheckpointAnnotation: "true",
				checkpointParentAnnotation: "foo/bar",
			}),
			expected: false,
		},
	}

	for _, tc := range cases {
		got := isValidParent(tc.pod)
		if tc.expected != got {
			t.Errorf("Expected: %t Got: %t For test: %s", tc.expected, got, tc.desc)
		}
	}
}

func TestIsCheckpoint(t *testing.T) {
	type testCase struct {
		desc     string
		pod      *v1.Pod
		expected bool
	}

	cases := []testCase{
		{
			desc: fmt.Sprintf("Pod is checkpoint and contains %s annotation key and value", checkpointParentAnnotation),
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{checkpointParentAnnotation: "podnamespace/podname"},
				},
			},
			expected: true,
		},
		{
			desc: fmt.Sprintf("Pod is checkpoint contains %s annotation key and no value", checkpointParentAnnotation),
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{checkpointParentAnnotation: ""},
				},
			},
			expected: true,
		},
		{
			desc: "Pod is not checkpoint & contains unrelated annotations",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"foo": "bar"},
				},
			},
			expected: false,
		},
		{
			desc: "Pod is not checkpoint & contains no annotations",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		got := isCheckpoint(tc.pod)
		if tc.expected != got {
			t.Errorf("Expected: %t Got: %t for test: %s", tc.expected, got, tc.desc)
		}
	}
}

func TestCopyPod(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podname",
			Namespace: "podnamespace",
		},
		Spec: v1.PodSpec{Containers: []v1.Container{{VolumeMounts: []v1.VolumeMount{{Name: "default-token"}}}}},
	}
	got, err := copyPod(&pod)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !apiequality.Semantic.DeepEqual(pod, *got) {
		t.Errorf("Expected:\n%#v\nGot:\n%#v", pod, got)
	}
}

func TestPodID(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podname",
			Namespace: "podnamespace",
		},
	}
	expected := "podnamespace/podname"
	got := podFullName(pod)
	if expected != got {
		t.Errorf("Expected: %s Got: %s", expected, got)
	}
}

func TestPodIDToInactiveCheckpointPath(t *testing.T) {
	id := "foo/bar"
	expected := inactiveCheckpointPath + "/foo-bar.json"
	got := podFullNameToInactiveCheckpointPath(id)
	if expected != got {
		t.Errorf("Expected: %s Got: %s", expected, got)
	}
}

func TestPodIDToActiveCheckpointPath(t *testing.T) {
	id := "foo/bar"
	expected := activeCheckpointPath + "/foo-bar.json"
	got := podFullNameToActiveCheckpointPath(id)
	if expected != got {
		t.Errorf("Expected: %s Got: %s", expected, got)
	}
}

func TestPodIDToSecretPath(t *testing.T) {
	id := "foo/bar"
	expected := checkpointSecretPath + "/foo/bar"
	got := podFullNameToSecretPath(id)
	if expected != got {
		t.Errorf("Expected %s Got %s", expected, got)
	}
}

func TestPodIDToConfigMapPath(t *testing.T) {
	id := "foo/bar"
	expected := checkpointConfigMapPath + "/foo/bar"
	got := podFullNameToConfigMapPath(id)
	if expected != got {
		t.Errorf("Expected %s Got %s", expected, got)
	}
}

func TestPodUserAndGroup(t *testing.T) {
	user1 := int64(1)
	user2 := int64(2)
	group1 := int64(10)
	for _, tc := range []struct {
		name    string
		pod     *v1.Pod
		wantUID int
		wantGID int
		wantErr bool
	}{{
		name:    "empty pod",
		pod:     &v1.Pod{},
		wantUID: rootUID,
	}, {
		name: "normal pod",
		pod: &v1.Pod{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name: "container",
				}},
			},
		},
		wantUID: rootUID,
	}, {
		name: "pod with PodSecurityContext",
		pod: &v1.Pod{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name: "container",
				}},
				SecurityContext: &v1.PodSecurityContext{RunAsUser: &user1, FSGroup: &group1},
			},
		},
		wantUID: int(user1),
		wantGID: int(group1),
	}, {
		name: "pod with Container.SecurityContext",
		pod: &v1.Pod{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name:            "container",
					SecurityContext: &v1.SecurityContext{RunAsUser: &user1},
				}},
			},
		},
		wantUID: int(user1),
	}, {
		name: "pod with matching SecurityContexts",
		pod: &v1.Pod{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name:            "container1",
					SecurityContext: &v1.SecurityContext{RunAsUser: &user1},
				}, {
					Name:            "container2",
					SecurityContext: &v1.SecurityContext{RunAsUser: &user1},
				}},
				SecurityContext: &v1.PodSecurityContext{RunAsUser: &user1, FSGroup: &group1},
			},
		},
		wantUID: int(user1),
		wantGID: int(group1),
	}, {
		name: "pod with conflicting PodSecurityContext and SecurityContext",
		pod: &v1.Pod{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name:            "container",
					SecurityContext: &v1.SecurityContext{RunAsUser: &user1},
				}},
				SecurityContext: &v1.PodSecurityContext{RunAsUser: &user2},
			},
		},
		wantErr: true,
	}, {
		name: "pod with conflicting SecurityContexts",
		pod: &v1.Pod{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name:            "container1",
					SecurityContext: &v1.SecurityContext{RunAsUser: &user1},
				}, {
					Name:            "container2",
					SecurityContext: &v1.SecurityContext{RunAsUser: &user2},
				}},
			},
		},
		wantErr: true,
	}} {
		uid, gid, err := podUserAndGroup(tc.pod)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s podUser() = err: %v, want: %v", tc.name, err != nil, tc.wantErr)
		} else if !tc.wantErr && (uid != tc.wantUID || gid != tc.wantGID) {
			t.Errorf("%s podUser() = %v, %v, want: %v, %v", tc.name, uid, gid, tc.wantUID, tc.wantGID)
		}
	}
}
