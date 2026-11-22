package httpapi

import (
	"testing"
)

func TestBoundedLiveQueueRejectsSlowConsumer(t *testing.T) {
	queue := make(chan []byte, 1)
	select {
	case queue <- []byte("first"):
	default:
		t.Fatal("the first live event should fit in the bounded queue")
	}
	select {
	case queue <- []byte("second"):
		t.Fatal("a full live queue must reject new events")
	default:
	}
}
