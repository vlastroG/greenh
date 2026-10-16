package core

import (
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestPrecheck(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	cal := Pot{DryRaw: ptr(540), WetRaw: ptr(270)}
	recent, old := now.Add(-10*time.Minute), now.Add(-31*time.Minute)
	if s := Precheck(cal, nil, now, 30*time.Minute); s != StatusSensorOffline {
		t.Error(s)
	}
	if s := Precheck(cal, &old, now, 30*time.Minute); s != StatusSensorOffline {
		t.Error(s)
	}
	if s := Precheck(Pot{}, &recent, now, 30*time.Minute); s != StatusNeedsCalibration {
		t.Error(s)
	}
	if s := Precheck(Pot{}, &old, now, 30*time.Minute); s != StatusSensorOffline {
		t.Error("offline wins over calibration")
	}
	if s := Precheck(cal, &recent, now, 30*time.Minute); s != StatusOK {
		t.Error(s)
	}
}
