/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// maxTargetTimestamp is the first instant of year 10000. Medusa (Python datetime) can't represent it or anything later.
var maxTargetTimestamp = time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)

// ParseTargetTimestamp parses a MedusaRestoreJob spec.targetTimestamp value and returns it in UTC.
// The value must be either an RFC3339 timestamp (for example 2024-07-15T12:30:00Z) or Unix epoch
// seconds, optionally with a fractional part (for example 1721046600.5). The result must fall within
// years 1970 to 9999.
func ParseTargetTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return checkTargetTimestampRange(s, t.UTC())
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		// Check the bounds before converting, as int64(f) is undefined for values out of the int64 range
		if f < 0 || f >= float64(maxTargetTimestamp.Unix()) {
			return checkTargetTimestampRange(s, time.Time{})
		}
		sec, frac := math.Modf(f)
		return checkTargetTimestampRange(s, time.Unix(int64(sec), int64(frac*float64(time.Second))).UTC())
	}
	return time.Time{}, fmt.Errorf("spec.targetTimestamp %q must be an RFC3339 timestamp or Unix epoch seconds", s)
}

func checkTargetTimestampRange(s string, t time.Time) (time.Time, error) {
	if t.Year() < 1970 || !t.Before(maxTargetTimestamp) {
		return time.Time{}, fmt.Errorf("spec.targetTimestamp %q must be between years 1970 and 9999", s)
	}
	return t, nil
}

// ValidateRestoreJobSpec checks the rules for a MedusaRestoreJobSpec that the CRD schema cannot express:
// exactly one of spec.backup and spec.targetTimestamp must be set, and spec.targetTimestamp, when set,
// must be parseable by ParseTargetTimestamp.
//
// It is used by the MedusaRestoreJob validating webhook and by the MedusaRestoreJob reconciler, so both
// apply the same rules.
func ValidateRestoreJobSpec(spec *MedusaRestoreJobSpec) error {
	hasBackup := spec.Backup != ""
	hasTimestamp := spec.TargetTimestamp != ""

	switch {
	case hasBackup && hasTimestamp:
		return fmt.Errorf("spec.backup and spec.targetTimestamp are mutually exclusive, set only one of them")
	case !hasBackup && !hasTimestamp:
		return fmt.Errorf("one of spec.backup or spec.targetTimestamp must be set")
	case hasTimestamp:
		if _, err := ParseTargetTimestamp(spec.TargetTimestamp); err != nil {
			return err
		}
	}
	return nil
}
