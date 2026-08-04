package cmd

import "testing"

func TestValidIP4(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"8.8.8.8", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"192.168.1.1", true},
		{" 1.2.3.4 ", true},
		{"256.1.1.1", false},
		{"1.2.3", false},
		{"1.2.3.4.5", false},
		{"example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := validIP4(c.in); got != c.want {
			t.Errorf("validIP4(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
