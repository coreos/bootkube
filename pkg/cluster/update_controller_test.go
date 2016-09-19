package cluster

import (
	"testing"

	"github.com/coreos/bootkube/pkg/cluster/components"
)

// type Component interface {
// 	Name() string
// 	UpdateToVersion(*components.Version) error
// 	Priority() int
// 	Version() (*components.Version, error)
// }

type fakeComponent struct {
	name             string
	priority         int
	requestedVersion *components.Version
	currentVersion   *components.Version
	shouldUpdate     bool
}

func (fk *fakeComponent) Name() string {
	return fk.name
}

func (fk *fakeComponent) UpdateToVersion(v *components.Version) (bool, error) {
	fk.requestedVersion = v
	return fk.shouldUpdate, nil
}

func (fk *fakeComponent) Priority() int {
	return fk.priority
}

func (fk *fakeComponent) Version() (*components.Version, error) {
	return fk.currentVersion, nil
}

func newFakeComponent(name string, priority int, versionString string, shouldUpdate bool, t *testing.T) *fakeComponent {
	ver, err := components.ParseVersionFromImage(versionString)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeComponent{
		name:           name,
		priority:       priority,
		currentVersion: ver,
		shouldUpdate:   shouldUpdate,
	}
}

func TestHighestClusterVersion(t *testing.T) {
	comps := []Component{
		newFakeComponent("first", 1, "foo.io/bar/baz:v1.2.3", false, t),
		newFakeComponent("last", 2, "foo.io/bar/baz:v1.1.3", false, t),
	}
	highestVer, err := highestClusterVersion(comps)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := comps[0].Version()
	if !highestVer.Semver().EQ(expected.Semver()) {
		t.Fatalf("expected highest cluster version of %s, got: %s", expected.Semver(), highestVer.Semver())
	}
}

func TestComponentOrdering(t *testing.T) {
	comps := []Component{
		newFakeComponent("highest", 1, "foo.io/bar/baz:v1.2.3", false, t),
		newFakeComponent("lowest", 2, "foo.io/bar/baz:v1.1.3", false, t),
	}
	highestVer, err := highestClusterVersion(comps)
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		desiredVersionString string
		asc                  bool
		desc                 string
	}{
		{desiredVersionString: "v1.3.0", asc: true, desc: "List should be sorted ascending for new higher version"},
		{desiredVersionString: "v1.1.3", asc: false, desc: "List should be sorted descending for new version lower than highest, but equal to lower"},
		{desiredVersionString: "v1.0.3", asc: false, desc: "List should be sorted descneding for new version lower than all others"},
	}

	for _, tc := range testCases {
		t.Log(tc.desc)
		desiredVer, err := components.ParseVersionFromImage(tc.desiredVersionString)
		if err != nil {
			t.Fatal(err)
		}
		sorted := sortComponentsByPriority(highestVer, desiredVer, comps)
		if tc.asc {
			if sorted[0].Name() != "highest" {
				t.Fatalf("Expected highest priority to be first, instead got: %s", sorted[0].Name())
			}
		} else {
			if sorted[0].Name() != "lowest" {
				t.Fatalf("Expected lowest priority to be first, instead got: %s", sorted[0].Name())
			}
		}
	}
}

func TestExitAfterComponentUpdate(t *testing.T) {
	// Update controller should run until a component successfully updates,
	// and then it should stop. The reasoning here is that, it's possible
	// the updates take a long time, so we want to ensure we're always
	// converging on the correct version, even if something happens out-of-band
	// by a cluster admin during the upgrade.
	comps := []Component{
		newFakeComponent("highest", 1, "foo.io/bar/baz:v1.2.3", false, t),
		newFakeComponent("lowest", 2, "foo.io/bar/baz:v1.1.3", false, t),
	}
	uc := &UpdateController{}
}

func TestNodesAlwaysUpdatedLast(t *testing.T) {
}
