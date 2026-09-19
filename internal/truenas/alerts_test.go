package truenas

import (
	"encoding/json"
	"testing"
)

func TestParseTrueNASTime(t *testing.T) {
	// Date wrapper shape: {"$date": 1789769434000}
	rawWrapper := json.RawMessage(`{"$date": 1789769434000}`)
	ts := parseTrueNASTime(rawWrapper)
	if ts == nil || *ts != 1789769434 {
		t.Fatalf("expected 1789769434, got %v", ts)
	}

	// Raw millisecond integer
	rawMillis := json.RawMessage(`1789769434000`)
	ts2 := parseTrueNASTime(rawMillis)
	if ts2 == nil || *ts2 != 1789769434 {
		t.Fatalf("expected 1789769434, got %v", ts2)
	}

	// Raw second integer
	rawSec := json.RawMessage(`1789769434`)
	ts3 := parseTrueNASTime(rawSec)
	if ts3 == nil || *ts3 != 1789769434 {
		t.Fatalf("expected 1789769434, got %v", ts3)
	}

	// Null / empty
	if parseTrueNASTime(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
	if parseTrueNASTime(json.RawMessage("null")) != nil {
		t.Fatal("expected nil for null input")
	}
}

func TestAlertPriority(t *testing.T) {
	if alertPriority("CRITICAL") >= alertPriority("WARNING") {
		t.Fatal("CRITICAL should have higher priority than WARNING")
	}
	if alertPriority("WARNING") >= alertPriority("INFO") {
		t.Fatal("WARNING should have higher priority than INFO")
	}
}
