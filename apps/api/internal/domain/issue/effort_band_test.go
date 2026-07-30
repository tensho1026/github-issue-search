package issue

import (
	"errors"
	"testing"
)

func TestParseEffortBandValidatesClosedVocabulary(t *testing.T) {
	for _, value := range []EffortBand{
		EffortThirtyMinutes,
		EffortTwoHours,
		EffortHalfDay,
		EffortOneDay,
		EffortThreeDays,
	} {
		parsed, err := ParseEffortBand(string(value))
		if err != nil || parsed != value {
			t.Fatalf("ParseEffortBand(%q) = %q, %v", value, parsed, err)
		}
	}

	for _, value := range []string{"weekend", " half_day "} {
		if _, err := ParseEffortBand(value); !errors.Is(
			err,
			ErrInvalidSearchCriteria,
		) {
			t.Fatalf("ParseEffortBand(%q) error = %v", value, err)
		}
	}
}

func TestEffortBandIsAtMostUsesProductOrder(t *testing.T) {
	if !EffortThirtyMinutes.IsAtMost(EffortHalfDay) ||
		!EffortHalfDay.IsAtMost(EffortHalfDay) ||
		EffortOneDay.IsAtMost(EffortHalfDay) ||
		EffortBand("unknown").IsAtMost(EffortThreeDays) {
		t.Fatal("effort band ordering is incorrect")
	}
}
