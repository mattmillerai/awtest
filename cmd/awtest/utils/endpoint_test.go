package utils

import (
	"strings"
	"testing"
)

func TestValidateEndpointURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{"http with port", "http://localhost:4566", "http://localhost:4566", ""},
		{"https with path", "https://example.com:8443/aws/", "https://example.com:8443/aws/", ""},
		{"uppercase scheme", "HTTP://localhost:4566", "HTTP://localhost:4566", ""},
		{"whitespace trimmed", "  http://localhost:4566\n", "http://localhost:4566", ""},
		{"missing scheme", "localhost:4566", "", "scheme must be http or https"},
		{"no scheme at all", "example.com", "", "scheme must be http or https"},
		{"ftp", "ftp://x", "", "scheme must be http or https"},
		{"no host", "http://", "", "missing host"},
		{"no host with port", "http://:4566", "", "missing host"},
		{"garbage", "http://[::1", "", "invalid -endpoint-url"},
		{"empty", "", "", "empty"},
		{"blank", "   ", "", "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateEndpointURL(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEndpointURL(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tests := []struct {
		name     string
		flag     string
		env      map[string]string
		want     string
		wantErr  string
		wantErrW string
	}{
		{"flag wins over env", "http://flag:1", map[string]string{"AWS_ENDPOINT_URL": "http://env:2"}, "http://flag:1", "", ""},
		{"env only", "", map[string]string{"AWS_ENDPOINT_URL": "http://env:2"}, "http://env:2", "", ""},
		{"neither", "", nil, "", "", ""},
		{"bad env", "", map[string]string{"AWS_ENDPOINT_URL": "notaurl"}, "", "AWS_ENDPOINT_URL", "scheme"},
		{"bad flag", "ftp://x", nil, "", "flag -endpoint-url", "scheme"},
		{"bad flag not rescued by good env", "ftp://x", map[string]string{"AWS_ENDPOINT_URL": "http://env:2"}, "", "flag -endpoint-url", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveEndpointURL(tt.flag, env(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), tt.wantErrW) {
					t.Fatalf("err = %v, want containing %q and %q", err, tt.wantErr, tt.wantErrW)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
