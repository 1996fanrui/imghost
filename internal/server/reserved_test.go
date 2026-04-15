package server

import "testing"

func TestIsReserved(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/swagger", true},
		{"/swagger/", true},
		{"/swagger/index.html", true},
		{"/swagger/doc.json", true},
		{"/swaggerX", false},
		{"/swag", false},
		{"/foo/swagger/", false},
		{"/foo", false},
		{"/", false},
		{"/swaggers", false},
	}
	for _, c := range cases {
		if got := IsReserved(c.path); got != c.want {
			t.Errorf("IsReserved(%q) = %v want %v", c.path, got, c.want)
		}
	}
}
