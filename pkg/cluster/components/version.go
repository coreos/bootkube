package components

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
)

// Version represents versioned cluster information,
// including the semver information and container
// image information.
type Version struct {
	// Semver is the semver parsed version for comparisons.
	Semver semver.Version
	// Image is the container image for this version.
	Image *ContainerImage
}

// ContainerImage describes a container image. It holds
// the repo / tag for the image.
type ContainerImage struct {
	// Repo is the repository this image is in.
	Repo string
	// Tag is the image tag.
	Tag string
}

// String returns a stringified version of the Containerimage
// in the format of $repo:$tag.
func (ci *ContainerImage) String() string {
	return fmt.Sprintf("%s:%s", ci.Repo, ci.Tag)
}

func ParseVersionFromImage(image string) (*Version, error) {
	var repo, tag string
	version := image
	if strings.Contains(image, ":") {
		parts := strings.SplitN(image, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unable to parse version: %s", image)
		}
		repo, tag = parts[0], parts[1]
		version = tag
	}
	if strings.Contains(version, "_") {
		version = strings.Replace(version, "_", "+", -1)
	}
	sv, err := semver.Parse(version)
	if err != nil {
		return nil, err
	}
	return &Version{Semver: sv, Image: &ContainerImage{Repo: repo, Tag: tag}}, nil
}
