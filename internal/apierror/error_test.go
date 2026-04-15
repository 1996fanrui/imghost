package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	rr := httptest.NewRecorder()
	Write(rr, http.StatusTeapot, "oops", "hi")
	if rr.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	var b Response
	if err := json.Unmarshal(rr.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.Error != "oops" || b.Message != "hi" {
		t.Fatalf("Response = %+v", b)
	}
}

func TestHelpers(t *testing.T) {
	cases := []struct {
		name   string
		fn     func(http.ResponseWriter, string)
		status int
		code   string
	}{
		{"bad", BadRequest, 400, "bad_request"},
		{"unauth", Unauthorized, 401, "unauthorized"},
		{"forbidden", Forbidden, 403, "forbidden"},
		{"notfound", NotFound, 404, "not_found"},
		{"mna", MethodNotAllowed, 405, "method_not_allowed"},
		{"internal", InternalError, 500, "internal_error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			c.fn(rr, "msg")
			if rr.Code != c.status {
				t.Fatalf("status = %d want %d", rr.Code, c.status)
			}
			var b Response
			if err := json.Unmarshal(rr.Body.Bytes(), &b); err != nil {
				t.Fatal(err)
			}
			if b.Error != c.code || b.Message != "msg" {
				t.Fatalf("Response = %+v", b)
			}
		})
	}
}
