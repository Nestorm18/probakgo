package ratelimit

import (
	"testing"
	"time"
)

func TestAllowKeyLimitsIndependently(t *testing.T) {
	l := New(2, time.Minute)

	if !l.AllowKey("key-a") || !l.AllowKey("key-a") {
		t.Fatal("first two requests for key-a should be allowed")
	}
	if l.AllowKey("key-a") {
		t.Fatal("third request for key-a should be limited")
	}
	if !l.AllowKey("key-b") {
		t.Fatal("key-b should have a separate bucket")
	}
}
