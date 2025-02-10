package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClientFromEnvironment(t *testing.T) {
	type testCase struct {
		value  string
		expect string
		err    error
	}

	testCases := map[string]*testCase{
		"empty":                      {value: "", expect: "http://127.0.0.1:11434"},
		"only address":               {value: "1.2.3.4", expect: "http://1.2.3.4:11434"},
		"only port":                  {value: ":1234", expect: "http://:1234"},
		"address and port":           {value: "1.2.3.4:1234", expect: "http://1.2.3.4:1234"},
		"scheme http and address":    {value: "http://1.2.3.4", expect: "http://1.2.3.4:80"},
		"scheme https and address":   {value: "https://1.2.3.4", expect: "https://1.2.3.4:443"},
		"scheme, address, and port":  {value: "https://1.2.3.4:1234", expect: "https://1.2.3.4:1234"},
		"hostname":                   {value: "example.com", expect: "http://example.com:11434"},
		"hostname and port":          {value: "example.com:1234", expect: "http://example.com:1234"},
		"scheme http and hostname":   {value: "http://example.com", expect: "http://example.com:80"},
		"scheme https and hostname":  {value: "https://example.com", expect: "https://example.com:443"},
		"scheme, hostname, and port": {value: "https://example.com:1234", expect: "https://example.com:1234"},
		"trailing slash":             {value: "example.com/", expect: "http://example.com:11434"},
		"trailing slash port":        {value: "example.com:1234/", expect: "http://example.com:1234"},
	}

	for k, v := range testCases {
		t.Run(k, func(t *testing.T) {
			t.Setenv("OLLAMA_HOST", v.value)

			client, err := ClientFromEnvironment()
			if err != v.err {
				t.Fatalf("expected %s, got %s", v.err, err)
			}

			if client.base.String() != v.expect {
				t.Fatalf("expected %s, got %s", v.expect, client.base.String())
			}
		})
	}
}

func TestClientStreamErrorResponse(t *testing.T) {
	testCases := []struct {
		name        string
		statusCode  int
		response    string
		wantErr     bool
		errorType   any // type to check error against (if specified)
		wantMessage string
	}{
		{
			name:        "error with message",
			statusCode:  http.StatusBadRequest,
			response:    `{"error": "test error message"}`,
			wantErr:     true,
			errorType:   &StatusError{},
			wantMessage: "test error message",
		},
		{
			name:        "error without message",
			statusCode:  http.StatusInternalServerError,
			response:    `{}`,
			wantErr:     true,
			errorType:   &StatusError{},
			wantMessage: "",
		},
		{
			name:        "error with ok status code",
			statusCode:  http.StatusOK,
			response:    `{"error": "test error message"}`,
			wantErr:     true,
			wantMessage: "test error message",
		},
		{
			name:       "no error with ok status code",
			statusCode: http.StatusOK,
			response:   `{"response": "ok"}`,
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up test server to simulate API responses
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				fmt.Fprintln(w, tc.response)
			}))
			defer ts.Close()

			client := NewClient(&url.URL{Scheme: "http", Host: ts.Listener.Addr().String()}, http.DefaultClient)

			// Test stream method with a no-op callback
			err := client.stream(context.Background(), http.MethodGet, "/test", nil, func([]byte) error {
				return nil
			})

			// Verify error behavior matches expectations
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tc.errorType != nil {
					if !errors.As(err, &tc.errorType) {
						t.Errorf("expected error of type %T, got %T", tc.errorType, err)
						return
					}
				}
				if statusErr, ok := err.(*StatusError); ok && statusErr.ErrorMessage != tc.wantMessage {
					t.Errorf("expected error message %q, got %q", tc.wantMessage, statusErr.ErrorMessage)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
