package permission

import "testing"

func TestParse(t *testing.T) {
	for _, in := range []string{"public", "private"} {
		got, err := Parse(in)
		if err != nil || string(got) != in {
			t.Fatalf("Parse(%q) = %v, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "Public", "secret", "PRIVATE"} {
		if _, err := Parse(in); err == nil {
			t.Fatalf("Parse(%q) expected error", in)
		}
	}
}
