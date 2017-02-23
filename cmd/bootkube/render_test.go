package main

import (
	"net"
	"testing"
)

func TestOffsetIP(t *testing.T) {
	cases := []struct {
		input    string
		offset   int
		expected string
	}{
		{"10.3.0.0/24", 1, "10.3.0.1"},
		{"10.3.0.0/24", 10, "10.3.0.10"},
		{"10.3.0.0/24", 15, "10.3.0.15"},
		{"10.3.0.0/16", 1, "10.3.0.1"},
		{"10.3.0.0/16", 10, "10.3.0.10"},
		{"10.3.0.0/16", 15, "10.3.0.15"},
		{"10.33.1.200/16", 1, "10.33.0.1"},
		{"10.33.1.200/16", 10, "10.33.0.10"},
		{"10.33.1.200/16", 15, "10.33.0.15"},
		{"192.168.1.0/24", 15, "192.168.1.15"},
	}

	for _, c := range cases {
		_, cidr, err := net.ParseCIDR(c.input)
		if err != nil {
			t.Errorf("unexpected CIDR parse error: %v", err)
		}
		ip, err := offsetServiceIP(cidr, c.offset)
		if ip.String() != c.expected {
			t.Errorf("expected %s, got %s", c.expected, ip.String())
		}
	}
}
