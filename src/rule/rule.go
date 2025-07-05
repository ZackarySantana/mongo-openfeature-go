package rule

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"net"
	"reflect"
	"regexp"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	cron "github.com/robfig/cron/v3"
)

// ExactMatchRule fires if ctx[Key] deep‐equals ValueData.
type ExactMatchRule struct {
	Key      string
	KeyValue string

	VariantID string
	ValueData any
}

func (r *ExactMatchRule) Matches(ctx map[string]any) bool {
	v, ok := ctx[r.Key]
	return ok && (v == r.KeyValue || reflect.DeepEqual(v, r.KeyValue))
}

func (r *ExactMatchRule) Value() any      { return r.ValueData }
func (r *ExactMatchRule) Variant() string { return r.VariantID }

// RegexRule fires if ctx[Key] (string) matches Pattern.
type RegexRule struct {
	Key           string
	RegexpPattern string
	// Regexp is not serialized, but compiled on demand.
	// This is to avoid the overhead of compiling the regex on every match.
	Regexp *regexp.Regexp `json:"-" bson:"-"`

	VariantID string
	ValueData any
}

func (r *RegexRule) Matches(ctx map[string]any) bool {
	v, ok := ctx[r.Key]
	if !ok {
		return false
	}
	// TODO: Test if we are compiling the regex too often?
	if r.Regexp == nil {
		var err error
		r.Regexp, err = regexp.Compile(r.RegexpPattern)
		if err != nil {
			slog.Error("invalid regex pattern", "key", r.Key, "pattern", r.RegexpPattern, "error", err)
			return false
		}
	}
	s, ok := v.(string)
	return ok && r.Regexp.MatchString(s)
}

func (r *RegexRule) Value() any      { return r.ValueData }
func (r *RegexRule) Variant() string { return r.VariantID }

// ExistsRule fires if ctx contains Key at all.
type ExistsRule struct {
	Key string

	VariantID string
	ValueData any
}

func (r *ExistsRule) Matches(ctx map[string]any) bool {
	_, ok := ctx[r.Key]
	return ok
}

func (r *ExistsRule) Value() any      { return r.ValueData }
func (r *ExistsRule) Variant() string { return r.VariantID }

// FractionalRule fires a percentage of the time (deterministic via FNV+salt).
type FractionalRule struct {
	Key        string
	Percentage float64 // in [0.0,100.0)

	VariantID string
	ValueData any
}

func (r *FractionalRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key]
	if !ok {
		return false
	}
	h := fnv.New32a()
	fmt.Fprint(h, r.Key, raw)
	bucket := h.Sum32() % 100
	return float64(bucket) < r.Percentage
}

func (r *FractionalRule) Value() any      { return r.ValueData }
func (r *FractionalRule) Variant() string { return r.VariantID }

// RangeRule fires if ctx[Key] falls between Min and Max.
type RangeRule struct {
	Key                        string
	Min, Max                   float64
	ExclusiveMin, ExclusiveMax bool

	VariantID string
	ValueData any
}

func (r *RangeRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key]
	if !ok {
		return false
	}
	var v float64
	switch x := raw.(type) {
	case int:
		v = float64(x)
	case int32:
		v = float64(x)
	case int64:
		v = float64(x)
	case float32:
		v = float64(x)
	case float64:
		v = x
	default:
		return false
	}
	if r.ExclusiveMin {
		if v <= r.Min {
			return false
		}
	} else {
		if v < r.Min {
			return false
		}
	}
	if r.ExclusiveMax {
		return v < r.Max
	}
	return v <= r.Max
}

func (r *RangeRule) Value() any      { return r.ValueData }
func (r *RangeRule) Variant() string { return r.VariantID }

// InListRule fires if ctx[Key] is deep equal to one of Items.
type InListRule struct {
	Key   string
	Items []any

	VariantID string
	ValueData any
}

func (r *InListRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key]
	if !ok {
		return false
	}
	for _, item := range r.Items {
		if raw == item || reflect.DeepEqual(raw, item) {
			return true
		}
	}
	return false
}

func (r *InListRule) Value() any      { return r.ValueData }
func (r *InListRule) Variant() string { return r.VariantID }

// PrefixRule fires if ctx[Key] (string) has the given prefix.
type PrefixRule struct {
	Key    string
	Prefix string

	VariantID string
	ValueData any
}

func (r *PrefixRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key]
	if !ok {
		return false
	}
	stringData, ok := raw.(string)
	return ok && strings.HasPrefix(stringData, r.Prefix)
}

func (r *PrefixRule) Value() any      { return r.ValueData }
func (r *PrefixRule) Variant() string { return r.VariantID }

// SuffixRule fires if ctx[Key] (string) has the given suffix.
type SuffixRule struct {
	Key    string
	Suffix string

	VariantID string
	ValueData any
}

func (r *SuffixRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key]
	if !ok {
		return false
	}
	stringData, ok := raw.(string)
	return ok && strings.HasSuffix(stringData, r.Suffix)
}

func (r *SuffixRule) Value() any      { return r.ValueData }
func (r *SuffixRule) Variant() string { return r.VariantID }

// ContainsRule fires if ctx[Key] (string) contains the given substring.
type ContainsRule struct {
	Key       string
	Substring string

	VariantID string
	ValueData any
}

func (r *ContainsRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key]
	if !ok {
		return false
	}
	stringData, ok := raw.(string)
	return ok && strings.Contains(stringData, r.Substring)
}

func (r *ContainsRule) Value() any      { return r.ValueData }
func (r *ContainsRule) Variant() string { return r.VariantID }

// IPRangeRule fires if ctx[Key] (string) parses as an IP in any of CIDRs.
type IPRangeRule struct {
	Key   string
	CIDRs []string

	VariantID string
	ValueData any
}

func (r *IPRangeRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key].(string)
	if !ok {
		return false
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return false
	}
	for _, cidr := range r.CIDRs {
		if _, netw, err := net.ParseCIDR(cidr); err == nil && netw.Contains(ip) {
			return true
		}
	}
	return false
}

func (r *IPRangeRule) Value() any      { return r.ValueData }
func (r *IPRangeRule) Variant() string { return r.VariantID }

// GeoFenceRule fires if two coordinates in ctx are within RadiusMeters
// of the center (using a simple haversine).
type GeoFenceRule struct {
	LatKey, LngKey       string
	LatCenter, LngCenter float64
	RadiusMeters         float64

	VariantID string
	ValueData any
}

func degToRad(d float64) float64 {
	return d * math.Pi / 180.0
}

func (r *GeoFenceRule) Matches(ctx map[string]any) bool {
	rawLat, okLat := ctx[r.LatKey]
	rawLng, okLng := ctx[r.LngKey]
	if !okLat || !okLng {
		return false
	}

	// Support int, float32, float64
	var lat, lng float64
	switch v := rawLat.(type) {
	case int:
		lat = float64(v)
	case float32:
		lat = float64(v)
	case float64:
		lat = v
	default:
		return false
	}
	switch v := rawLng.(type) {
	case int:
		lng = float64(v)
	case float32:
		lng = float64(v)
	case float64:
		lng = v
	default:
		return false
	}

	// Haversine formula
	const earthRadius = 6371000.0 // meters

	latRad1 := degToRad(lat)
	latRad2 := degToRad(r.LatCenter)
	deltaLat := degToRad(r.LatCenter - lat)
	deltaLng := degToRad(r.LngCenter - lng)

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(latRad1)*math.Cos(latRad2)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := earthRadius * c
	return distance <= r.RadiusMeters
}

func (r *GeoFenceRule) Value() any      { return r.ValueData }
func (r *GeoFenceRule) Variant() string { return r.VariantID }

// DateTimeRule fires if ctx[Key] (time.Time) is between After and Before.
type DateTimeRule struct {
	Key    string
	After  time.Time
	Before time.Time

	VariantID string
	ValueData any
}

func (r *DateTimeRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key].(time.Time)
	if !ok {
		return false
	}
	return raw.After(r.After) && raw.Before(r.Before)
}

func (r *DateTimeRule) Value() any      { return r.ValueData }
func (r *DateTimeRule) Variant() string { return r.VariantID }

type SemVerRule struct {
	Key        string
	Constraint string // e.g., ">= 1.2.3, < 2.0.0" or "~2.3.4"

	VariantID string
	ValueData any
}

func (r *SemVerRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key].(string)
	if !ok {
		return false
	}

	c, err := semver.NewConstraint(r.Constraint)
	if err != nil {
		slog.Error("invalid semver constraint", "constraint", r.Constraint, "error", err)
		return false
	}

	v, err := semver.NewVersion(raw)
	if err != nil {
		// The value in the context is not a valid version string, so it cannot match.
		return false
	}

	// Check if the version satisfies the constraint.
	return c.Check(v)
}

func (r *SemVerRule) Value() any      { return r.ValueData }
func (r *SemVerRule) Variant() string { return r.VariantID }

// CronRule fires if a time falls within a recurring window. The window starts
// at a time defined by the CronSpec and lasts for the specified Duration.
//
// The time to be checked can be provided in two ways:
//  1. From the context: If Key is set, the rule will look for a time.Time
//     value in ctx[Key].
//  2. From the system clock: If Key is an empty string (""), the rule will
//     use time.Now() as the time to check.
//
// **Important Note on Time-Sensitive Workflows:**
// Using the system clock (by leaving Key empty) is convenient but has drawbacks.
// It makes the rule's outcome non-deterministic and hard to test. A test might
// pass or fail depending on the exact moment it is run.
//
// For critical, reproducible, or easily testable workflows, it is highly
// recommended to pass a specific time via the context map. This allows you to
// control the "current" time during tests and ensures that re-evaluating the
// same context will always yield the same result.
type CronRule struct {
	Key      string        // Optional. If empty, time.Now() is used.
	CronSpec string        // e.g., "0 9 * * MON-FRI" for 9:00 AM on weekdays.
	Duration time.Duration // e.g., 8 * time.Hour for an 8-hour window.

	// schedule is not serialized, but compiled on demand from CronSpec.
	schedule cron.Schedule `json:"-" bson:"-"`

	VariantID string
	ValueData any
}

func (r *CronRule) Matches(ctx map[string]any) bool {
	var checkTime time.Time
	var ok bool

	// If Key is empty, use time.Now(). Otherwise, get the time from the context.
	if r.Key == "" {
		checkTime = time.Now()
		ok = true
	} else {
		var raw any
		raw, ok = ctx[r.Key]
		if ok {
			checkTime, ok = raw.(time.Time)
		}
	}

	if !ok {
		// This fails if the key was not found or the value was not a time.Time.
		return false
	}

	// Compile the cron schedule on first use (and cache it).
	if r.schedule == nil {
		p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		var err error
		r.schedule, err = p.Parse(r.CronSpec)
		if err != nil {
			slog.Error("invalid cron spec", "key", r.Key, "spec", r.CronSpec, "error", err)
			return false
		}
	}

	// Find the most recent activation time for the schedule.
	// We look for the next activation *after* the start of our potential window.
	windowStart := checkTime.Add(-r.Duration)
	previousOrNextActivation := r.schedule.Next(windowStart)

	if previousOrNextActivation.IsZero() {
		return false // No scheduled times found for this spec.
	}

	// The checkTime is inside the window if the last activation was not after it.
	return !previousOrNextActivation.After(checkTime)
}

func (r *CronRule) Value() any      { return r.ValueData }
func (r *CronRule) Variant() string { return r.VariantID }
