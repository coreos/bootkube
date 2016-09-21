package components

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
)

func TestDaemonSetUpdaterVersion(t *testing.T) {
	selector, err := unversioned.ParseToLabelSelector("k8s-app=test")
	if err != nil {
		t.Fatal(err)
	}
	testDS := &extensions.DaemonSet{
		ObjectMeta: api.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				updatePriorityAnnotation: "1",
			},
		},
		Spec: extensions.DaemonSetSpec{
			Selector: selector,
		},
	}
	testPods := []*api.Pod{
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name: "test-abcd",
				Labels: map[string]string{
					"k8s-app": "test",
				},
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Name:  "test",
						Image: "foo.bar.io/foo/baz:v1.3.0",
					},
				},
			},
		},
		&api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name: "test-efgh",
				Labels: map[string]string{
					"k8s-app": "test",
				},
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Name:  "test",
						Image: "foo.bar.io/foo/baz:v1.4.0",
					},
				},
			},
		},
	}

	dsStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	podStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	if err := dsStore.Add(testDS); err != nil {
		t.Fatal(err)
	}
	if err := podStore.Add(testPods[0]); err != nil {
		t.Fatal(err)
	}
	if err := podStore.Add(testPods[1]); err != nil {
		t.Fatal(err)
	}

	ds, err := NewDaemonSetUpdater(nil, testDS, cache.StoreToDaemonSetLister{dsStore}, StoreToPodLister{podStore})
	if err != nil {
		t.Fatal(err)
	}
	v, err := ds.Version()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := ParseVersionFromImage("foo.bar.io/foo/baz:v1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Semver().EQ(expected.Semver()) {
		t.Fatalf("wrong version returned, expected %s got %s", expected.Semver(), v.Semver())
	}
}
