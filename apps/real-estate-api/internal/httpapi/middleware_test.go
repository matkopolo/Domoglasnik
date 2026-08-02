package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSPreflightContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		origin       string
		method       string
		headers      string
		allowOrigin  bool
		allowMethod  bool
		allowHeaders bool
	}{
		{"login", "http://localhost:3000", "POST", "Content-Type", true, true, true},
		{"preferences", "http://localhost:3000", "PUT", "Authorization, Content-Type", true, true, true},
		{"anomaly review", "http://localhost:3000", "PATCH", "Authorization, Content-Type", true, true, true},
		{"disallowed origin", "https://evil.example", "POST", "Content-Type", false, false, false},
		{"unsupported method", "http://localhost:3000", "DELETE", "Authorization", true, false, true},
		{"unsupported header", "http://localhost:3000", "POST", "X-Evil", true, true, false},
	}
	handler := cors([]string{"http://localhost:3000"}, http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			t.Fatal("preflight reached application handler")
		},
	))
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", test.method)
			request.Header.Set("Access-Control-Request-Headers", test.headers)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			assertHeaderContains(t, response.Header().Get("Access-Control-Allow-Origin"),
				test.origin, test.allowOrigin)
			assertHeaderContains(t, response.Header().Get("Access-Control-Allow-Methods"),
				test.method, test.allowMethod)
			for _, header := range strings.Split(test.headers, ",") {
				assertHeaderContains(t, response.Header().Get("Access-Control-Allow-Headers"),
					strings.TrimSpace(header), test.allowHeaders)
			}
			vary := strings.Join(response.Header().Values("Vary"), ",")
			for _, value := range []string{
				"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers",
			} {
				if !strings.Contains(vary, value) {
					t.Errorf("Vary=%q missing %q", vary, value)
				}
			}
		})
	}
}

func TestStatusRecorderKeepsFirstStatus(t *testing.T) {
	response := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: response}
	recorder.WriteHeader(http.StatusCreated)
	recorder.WriteHeader(http.StatusInternalServerError)
	if recorder.status != http.StatusCreated || response.Code != http.StatusCreated {
		t.Fatalf("recorder=%d response=%d", recorder.status, response.Code)
	}
}

func assertHeaderContains(t *testing.T, actual, expected string, want bool) {
	t.Helper()
	contains := strings.Contains(strings.ToLower(actual), strings.ToLower(expected))
	if contains != want {
		t.Fatalf("header=%q contains %q: got %v want %v", actual, expected, contains, want)
	}
}
