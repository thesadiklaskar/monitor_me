package httpapi

import (
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
