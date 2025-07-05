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

	"github.com/open-feature/go-sdk/openfeature"
)

type Rule interface {
	Matches(ctx map[string]any) bool
	Value() any
	Variant() string
}

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
	LatKey, LngKey string
	CenterLat      float64
	CenterLng      float64
	RadiusMeters   float64
	VariantID      string
	ValueData      any
}

func (r *GeoFenceRule) Matches(ctx map[string]any) bool {
	latI, lok := ctx[r.LatKey]
	lngI, gok := ctx[r.LngKey]
	if !lok || !gok {
		return false
	}
	lat, lok2 := latI.(float64)
	lng, gok2 := lngI.(float64)
	if !lok2 || !gok2 {
		return false
	}
	// haversine:
	const R = 6371000.0 // earth radius in meters
	dLat := (r.CenterLat - lat) * math.Pi / 180
	dLng := (r.CenterLng - lng) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat*math.Pi/180)*math.Cos(r.CenterLat*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R*c <= r.RadiusMeters
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

// -----------------------------------------------------------------------------
// FlagDefinition (non‐generic) with its own Evaluate
// -----------------------------------------------------------------------------

type FlagDefinition struct {
	FlagName string

	DefaultValue   any
	DefaultVariant string

	Rules []ConcreteRule
}

// Evaluate walks the Rules in order, returns the first match’s (value,detail),
// or the default if none match.
func (def *FlagDefinition) Evaluate(ctx map[string]any) (any, openfeature.ProviderResolutionDetail) {
	for _, rule := range def.Rules {
		if rule.Matches(ctx) {
			return rule.Value(), openfeature.ProviderResolutionDetail{
				Reason:  openfeature.TargetingMatchReason,
				Variant: rule.Variant(),
			}
		}
	}
	return def.DefaultValue, openfeature.ProviderResolutionDetail{
		Reason:  openfeature.DefaultReason,
		Variant: def.DefaultVariant,
	}
}
