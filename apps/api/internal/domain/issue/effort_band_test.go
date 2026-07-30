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
		parsed, err := ParseEffortBand(" " + string(value) + " ")
		if err != nil || parsed != value {
			t.Fatalf("ParseEffortBand(%q) = %q, %v", value, parsed, err)
		}
	}

	if _, err := ParseEffortBand("weekend"); !errors.Is(
		err,
		ErrInvalidSearchCriteria,
	) {
		t.Fatalf("ParseEffortBand(invalid) error = %v", err)
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
