package store

import (
	"testing"
)

func TestParseTriggerToSeconds(t *testing.T) {
	tests := []struct {
		trigger string
		want    int
	}{
		{"-PT15M", -900},
		{"-PT1H", -3600},
		{"-PT15H", -54000},
		{"-P1D", -86400},
		{"-P1W", -604800},
		{"-PT30S", -30},
		{"-P1DT2H30M", -(86400 + 7200 + 1800)},
		{"PT15M", 900},
		{"PT0S", 0},
		{"", 0},
		{"-P2W", -1209600},
		{"-PT1H30M", -(3600 + 1800)},
	}

	for _, tt := range tests {
		t.Run(tt.trigger, func(t *testing.T) {
			got := parseTriggerToSeconds(tt.trigger)
			if got != tt.want {
				t.Errorf("parseTriggerToSeconds(%q) = %d, want %d", tt.trigger, got, tt.want)
			}
		})
	}
}
