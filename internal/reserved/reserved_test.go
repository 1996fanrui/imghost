package reserved

import "testing"

func TestIsName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"swagger", true},
		{"swaggerX", false},
		{"", false},
		{"photos", false},
	}
	for _, c := range cases {
		if got := IsName(c.name); got != c.want {
			t.Errorf("IsName(%q) = %v want %v", c.name, got, c.want)
		}
	}
}
