package webapp

import (
	"dslparser/internal/parseradapter"
	"dslparser/internal/pipeline"
	"dslparser/internal/workspace"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeHasSecurityAndAccessibility(t *testing.T) {
	runner := pipeline.New(parseradapter.New())
	app, err := New(workspace.NewScanner(runner), runner, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8580/", nil)
	request.Host = "127.0.0.1:8580"
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	body := response.Body.String()
	for _, want := range []string{"lang=\"ru\"", "<main", "role=\"status\"", "<label", "/shutdown"} {
		if !containsText(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if response.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}
func TestRejectsHostAndWrongMethod(t *testing.T) {
	runner := pipeline.New(parseradapter.New())
	app, _ := New(workspace.NewScanner(runner), runner, nil)
	r := httptest.NewRequest("GET", "http://evil/", nil)
	r.Host = "evil"
	w := httptest.NewRecorder()
	app.Handler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("GET", "http://127.0.0.1:8580/workspace", nil)
	r.Host = "127.0.0.1:8580"
	w = httptest.NewRecorder()
	app.Handler().ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
func containsText(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
