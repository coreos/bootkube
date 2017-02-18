package bootkube

import (
	"testing"
)

const (
	defaultServiceCIDR = "10.3.0.0/24"
	defaultPodCIDR     = "10.2.0.0/24"
)

func TestFindFlag(t *testing.T) {
	flags := []string{
		"--key1=value",
		"--key2 val",
		"--service-cluster-ip-range=10.3.0.0/24",
		"--cluster-cidr=10.2.0.0/24",
		"--foobar baz",
	}
	cases := []struct {
		flag     string
		expected string
	}{
		{"--service-cluster-ip-range", defaultServiceCIDR},
		{"--cluster-cidr", defaultPodCIDR},
		{"--key1", "value"},
		{"--key2", "val"},
		{"--missing-flag", ""},
		{"--foo", ""},
		{"--foobar", "baz"},
	}
	for _, c := range cases {
		if v := findFlag(c.flag, flags); v != c.expected {
			t.Errorf("exected %s, got %s", c.expected, v)
		}
	}
}
