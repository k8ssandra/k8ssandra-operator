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

// isoLocalLayout is ISO-8601 without a timezone designator. Medusa reads such a value as UTC.
const isoLocalLayout = "2006-01-02T15:04:05"

// ParseTargetTimestamp parses a MedusaRestoreJob spec.targetTimestamp value and returns it in UTC.
//
// Accepted formats, tried in this order, follow what Medusa accepts so the operator is never stricter than Medusa:
//   - ISO-8601 / RFC3339 with a timezone offset, for example 2024-07-15T12:30:00Z or 2024-07-15T12:30:00+02:00
//   - ISO-8601 without a timezone, for example 2024-07-15T12:30:00, which is read as UTC
//   - a Unix epoch in seconds as a float string, for example 1721046600.5
//
// Fractional seconds are accepted in the ISO-8601 forms.
func ParseTargetTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation(isoLocalLayout, s, time.UTC); err == nil {
		return t, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		sec, frac := math.Modf(f)
		return time.Unix(int64(sec), int64(frac*float64(time.Second))).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("spec.targetTimestamp %q is not a valid ISO-8601 timestamp or Unix epoch seconds value", s)
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
