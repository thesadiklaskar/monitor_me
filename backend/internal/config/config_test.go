package config

import (
	"strings"
	"testing"
)

func TestValidateDatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "postgres URL", value: "postgres://user:password@localhost:5432/monitor_me", wantErr: false},
		{name: "postgresql URL", value: "postgresql://user@localhost/monitor_me", wantErr: false},
		{name: "missing host", value: "postgres:///monitor_me", wantErr: true},
		{name: "missing database", value: "postgres://user@localhost", wantErr: true},
		{name: "unsupported scheme", value: "mysql://user@localhost/monitor_me", wantErr: true},
		{name: "missing user", value: "postgres://localhost/monitor_me", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDatabaseURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateDatabaseURL() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestParseWebAllowedOrigins(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "one origin", raw: "http://localhost:3000", want: []string{"http://localhost:3000"}},
		{name: "multiple origins", raw: "https://monitor.example.com, http://localhost:3000", want: []string{"https://monitor.example.com", "http://localhost:3000"}},
		{name: "duplicates removed", raw: "http://localhost:3000,http://localhost:3000", want: []string{"http://localhost:3000"}},
		{name: "path rejected", raw: "https://monitor.example.com/dashboard", wantErr: true},
		{name: "credentials rejected", raw: "https://user:pass@monitor.example.com", wantErr: true},
		{name: "non-http scheme rejected", raw: "ftp://monitor.example.com", wantErr: true},
		{name: "empty list rejected", raw: " , ", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseWebAllowedOrigins(test.raw)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseWebAllowedOrigins() error = %v, wantErr %t", err, test.wantErr)
			}
			if !test.wantErr && strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("parseWebAllowedOrigins() = %#v, want %#v", got, test.want)
			}
		})
	}
}
