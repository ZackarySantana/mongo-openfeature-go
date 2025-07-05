package rule

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExactMatchRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"ExactMatch": {
			ctx: map[string]any{
				"test_key": "test_value",
			},
			matches: true,
		},
		"DifferentValue": {
			ctx: map[string]any{
				"test_key": "different_value",
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &ExactMatchRule{
				Key:       "test_key",
				KeyValue:  "test_value",
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestRegexRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"RegexMatch1": {
			ctx: map[string]any{
				"test_key": "12345",
			},
			matches: true,
		},
		"RegexMatch2": {
			ctx: map[string]any{
				"test_key": "67890",
			},
			matches: true,
		},
		"DifferentValue": {
			ctx: map[string]any{
				"test_key": "abcde",
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &RegexRule{
				Key:           "test_key",
				RegexpPattern: `^\d{5}$`,
				VariantID:     "test_variant",
				ValueData:     "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestExistsRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"other_key": "other_value",
			},
			matches: false,
		},
		"Exists": {
			ctx: map[string]any{
				"test_key": "test_value",
			},
			matches: true,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &ExistsRule{
				Key:       "test_key",
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestFractionalRule(t *testing.T) {
	ctxs := []map[string]any{}
	for i := 0; i < 100000; i++ {
		ctxs = append(ctxs, map[string]any{
			"test_key": fmt.Sprintf("test_value_%f", rand.Float64()),
		})
	}

	for tName, tCase := range map[string]float64{
		"Never": 0,
		// We do not test 1% because of the high variance in small samples.
		"5Percent":   5,
		"25Percent":  25,
		"50Percent":  50,
		"75Percent":  75,
		"99Percent":  99,
		"100Percent": 100,
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &FractionalRule{
				Key:        "test_key",
				Percentage: tCase,
				VariantID:  "test_variant",
				ValueData:  "test_value_data",
			}
			matchesCount := 0
			for _, ctx := range ctxs {
				if rule.Matches(ctx) {
					matchesCount++
				}
			}
			expectedMatches := float64(len(ctxs)) * tCase / 100.0
			allowedDeviation := expectedMatches * 0.05

			assert.InDelta(t, matchesCount, expectedMatches, allowedDeviation, "Expected matches deviation exceeded")
		})
	}
}

func TestRangeRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"InRange": {
			ctx: map[string]any{
				"test_key": 50.0,
			},
			matches: true,
		},
		"BelowMin": {
			ctx: map[string]any{
				"test_key": 9.9,
			},
			matches: false,
		},
		"AboveMax": {
			ctx: map[string]any{
				"test_key": 100.1,
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &RangeRule{
				Key:          "test_key",
				Min:          10.0,
				Max:          100.0,
				ExclusiveMin: false,
				ExclusiveMax: false,
				VariantID:    "test_variant",
				ValueData:    "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestInListRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"InList": {
			ctx: map[string]any{
				"test_key": "value2",
			},
			matches: true,
		},
		"NotInList": {
			ctx: map[string]any{
				"test_key": "value4",
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &InListRule{
				Key:       "test_key",
				Items:     []any{"value1", "value2", "value3"},
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestPrefixRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"HasPrefix": {
			ctx: map[string]any{
				"test_key": "prefix_value",
			},
			matches: true,
		},
		"NoPrefix": {
			ctx: map[string]any{
				"test_key": "other_value",
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &PrefixRule{
				Key:       "test_key",
				Prefix:    "prefix_",
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestSuffixRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"HasSuffix": {
			ctx: map[string]any{
				"test_key": "value_suffix",
			},
			matches: true,
		},
		"NoSuffix": {
			ctx: map[string]any{
				"test_key": "other_value",
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &SuffixRule{
				Key:       "test_key",
				Suffix:    "_suffix",
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestContainsRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"Contains": {
			ctx: map[string]any{
				"test_key": "value_contains",
			},
			matches: true,
		},
		"NoContains": {
			ctx: map[string]any{
				"test_key": "other_value",
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &ContainsRule{
				Key:       "test_key",
				Substring: "contains",
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestIPRangeRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"InRange": {
			ctx: map[string]any{
				"test_key": "192.168.1.1",
			},
			matches: true,
		},
		"OutOfRange": {
			ctx: map[string]any{
				"test_key": "10.0.0.1",
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &IPRangeRule{
				Key:       "test_key",
				CIDRs:     []string{"192.168.1.0/24"},
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestGeoFenceRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"InRange": {
			ctx: map[string]any{
				"lat": 37.7749,
				"lng": -122.4194,
			},
			matches: true,
		},
		"OutOfRange": {
			ctx: map[string]any{
				"lat": 34.0522,
				"lng": -118.2437,
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &GeoFenceRule{
				LatKey:       "lat",
				LngKey:       "lng",
				LatCenter:    37.7749,
				LngCenter:    -122.4194,
				RadiusMeters: 1000.0, // 1 km radius
				VariantID:    "test_variant",
				ValueData:    "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestDateTimeRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"NotFound": {
			ctx: map[string]any{
				"some_key": "other_value",
			},
			matches: false,
		},
		"InRange": {
			ctx: map[string]any{
				"test_key": time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
			},
			matches: true,
		},
		"BeforeRange": {
			ctx: map[string]any{
				"test_key": time.Date(2023, 9, 30, 12, 0, 0, 0, time.UTC),
			},
			matches: false,
		},
		"AfterRange": {
			ctx: map[string]any{
				"test_key": time.Date(2023, 10, 2, 12, 0, 0, 0, time.UTC),
			},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &DateTimeRule{
				Key:       "test_key",
				After:     time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC),
				Before:    time.Date(2023, 10, 2, 0, 0, 0, 0, time.UTC),
				VariantID: "test_variant",
				ValueData: "test_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}
