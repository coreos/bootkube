package components

import (
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/version"

	"github.com/blang/semver"
)

const updatePriorityAnnotation = "update-controller.alpha.coreos.com/priority"

func noAnnotationError(component, name string) error {
	return fmt.Errorf("no priority annotation for %s %s", component, name)
}

// Version represents versioned cluster information,
// including the semver information and container
// image information.
type Version struct {
	// Semver is the semver parsed version for comparisons.
	semver semver.Version
	// Image is the container image for this version.
	image *ContainerImage
}

func ParseVersionFromImage(image string) (*Version, error) {
	var repo, tag string
	ver := image
	if strings.Contains(image, ":") {
		parts := strings.SplitN(image, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unable to parse version: %s", image)
		}
		repo, tag = parts[0], parts[1]
		ver = tag
	}
	if strings.Contains(ver, "_") {
		ver = strings.Replace(ver, "_", "+", -1)
	}
	sv, err := version.Parse(ver)
	if err != nil {
		return nil, err
	}
	return &Version{semver: sv, image: &ContainerImage{repo: repo, tag: tag}}, nil
}

// Semver returns a Semver object for version comparisons.
func (v *Version) Semver() semver.Version {
	return v.semver
}

// ContainerImage describes a container image. It holds
// the repo / tag for the image.
type ContainerImage struct {
	// Repo is the repository this image is in.
	repo string
	// Tag is the image tag.
	tag string
}

// Tag returns the tag for the image.
func (ci *ContainerImage) Tag() string {
	return ci.tag
}

// String returns a stringified version of the Containerimage
// in the format of $repo:$tag.
func (ci *ContainerImage) String() string {
	return fmt.Sprintf("%s:%s", ci.repo, ci.tag)
}
