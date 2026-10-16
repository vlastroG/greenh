package core

import "time"

// Precheck returns a non-ok status when the pot can't be advised on yet.
// Offline takes priority over missing calibration.
func Precheck(p Pot, lastReadingAt *time.Time, now time.Time, offlineAfter time.Duration) Status {
	if lastReadingAt == nil || now.Sub(*lastReadingAt) > offlineAfter {
		return StatusSensorOffline
	}
	if !p.Calibrated() {
		return StatusNeedsCalibration
	}
	return StatusOK
}
