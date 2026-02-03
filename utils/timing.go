package utils

import (
	"math"
	"time"
)

/*
*
GetProgress to track state of a progress bar

	@param startTime (time.Time) - the time at which an event started
	@param expectedDuration (time.Duration) - how long the event is expected to last
	@return (float64) - the proportion of status bar completion
*/
func GetProgress(startTime time.Time, expectedDuration time.Duration) float64 {
	elapsed := time.Since(startTime)
	if elapsed <= 0 {
		return 0
	}
	if expectedDuration <= 0 {
		return 1
	}
	if elapsed <= expectedDuration {
		return 0.8 * (float64(elapsed) / float64(expectedDuration))
	}

	over := elapsed - expectedDuration
	ratio := float64(over) / float64(expectedDuration)
	return 1 - 0.2*math.Exp(-ratio)
}

/*
EstimateInstanceRestorationDuration returns the total duration of how long a snapshot would take to restore

	@param snapshotSize (int64) - the size of the snapshot (in bytes)
	@return (time.Duration) - the duration of the restoration
*/
func EstimateInstanceRestorationDuration(snapshotSize int64) time.Duration {
	if snapshotSize <= 0 {
		return 5 * time.Minute
	}

	const bytesPerGiB = int64(1 << 30)
	const perGiB = 6 * time.Second

	whole := (snapshotSize / bytesPerGiB) * int64(perGiB)
	rem := (snapshotSize % bytesPerGiB) * int64(perGiB) / bytesPerGiB
	return 5*time.Minute + time.Duration(whole+rem)
}
