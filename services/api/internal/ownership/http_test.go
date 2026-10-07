package ownership

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNameValidationAndBodyBound(t *testing.T) {
	cases := []struct {
		body   string
		status int
	}{
		{`{"name":"  My API  "}`, 0}, {`{"name":"` + strings.Repeat("🙂", 100) + `"}`, 0},
		{`{}`, 400}, {`{"name":null}`, 400}, {`{"name":1}`, 400}, {`{"name":" "}`, 400},
		{`{"name":"a\nb"}`, 400}, {`{"name":"x","ownerId":"evil"}`, 400}, {`{"name":"x","name":"y"}`, 400},
		{`{"name":"x"} {}`, 400}, {`[]`, 400}, {`{"name":"` + strings.Repeat("x", 101) + `"}`, 400},
		{`{"name":"` + strings.Repeat("x", 8192) + `"}`, 413}, {"{\"name\":\"\xff\"}", 400},
	}
	for _, tc := range cases {
		t.Run(tc.body[:min(len(tc.body), 24)], func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			name, err := readName(httptest.NewRecorder(), r)
			if tc.status != 0 {
				assertStatus(t, err, tc.status)
			} else if err != nil || name == "" || strings.TrimSpace(name) != name {
				t.Fatalf("name=%q err=%v", name, err)
			}
		})
	}
	r := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(`{"name":"x"}`))
	_, err := readName(httptest.NewRecorder(), r)
	assertStatus(t, err, 415)
}
func TestPreconditions(t *testing.T) {
	r := httptest.NewRequest("PATCH", "/api/v1/projects/x", nil)
	_, err := precondition(r)
	assertStatus(t, err, 428)
	for _, tag := range []string{"0", `W/"0"`, `"01"`, `"-1"`, `"0", "1"`, "*", `"9223372036854775807"`} {
		r.Header.Set("If-Match", tag)
		_, err = precondition(r)
		assertStatus(t, err, 400)
	}
	r.Header.Set("If-Match", `"0"`)
	v, err := precondition(r)
	if v != 0 || err != nil {
		t.Fatal(v, err)
	}
	r.Header.Add("If-Match", `"1"`)
	_, err = precondition(r)
	assertStatus(t, err, 400)
}
func TestErrorDoesNotExposeUnderlyingFailure(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, errors.New("private-token database-details"))
	if w.Code != 500 || strings.Contains(w.Body.String(), "private-token") || !strings.Contains(w.Body.String(), `"details":[]`) {
		t.Fatal(w.Body.String())
	}
}
