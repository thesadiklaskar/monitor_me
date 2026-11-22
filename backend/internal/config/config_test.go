package config

import "testing"

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
