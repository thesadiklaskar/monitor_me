package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundedLiveQueueRejectsSlowConsumer(t *testing.T) {
	queue := make(chan []byte, 1)
	if !enqueueLivePayload(queue, []byte("first")) {
		t.Fatal("the first live event should fit in the bounded queue")
	}
	if enqueueLivePayload(queue, []byte("second")) {
		t.Fatal("a full live queue must reject new events")
	}
}

func TestLiveOriginAllowlistSeparatesAllowedAndUntrustedOrigins(t *testing.T) {
	api := &API{allowedLiveOrigins: map[string]struct{}{"http://localhost:3000": {}}}
	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "configured dashboard origin", origin: "http://localhost:3000", want: true},
		{name: "same origin", origin: "http://localhost:8080", want: true},
		{name: "scheme mismatch", origin: "https://localhost:8080", want: false},
		{name: "untrusted host", origin: "https://attacker.example", want: false},
		{name: "missing origin", origin: "", want: false},
		{name: "origin path rejected", origin: "http://localhost:3000/evil", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://localhost:8080/v1/live/ws", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if got := api.liveOriginAllowed(request); got != test.want {
				t.Fatalf("liveOriginAllowed() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestLiveOriginUsesForwardedHTTPSOrigin(t *testing.T) {
	api := &API{}
	request := httptest.NewRequest(http.MethodGet, "http://api:8080/v1/live/ws", nil)
	request.Host = "monitor.example.com"
	request.Header.Set("Origin", "https://monitor.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	if !api.liveOriginAllowed(request) {
		t.Fatal("the reverse-proxy public HTTPS origin should be accepted")
	}
}
