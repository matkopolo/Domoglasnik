package analytics

import "testing"

func TestValuationAssessmentUsesSlovenianWarnings(t *testing.T) {
	tests := []struct {
		name        string
		medianAPE   *float64
		reliability string
		warning     string
	}{
		{name: "missing error", reliability: "very_low", warning: "Napaka modela za to skupino nepremičnin ni na voljo."},
		{name: "moderate", medianAPE: floatPointer(0.2), reliability: "moderate"},
		{name: "low", medianAPE: floatPointer(0.4), reliability: "low"},
		{name: "excessive error", medianAPE: floatPointer(0.6), reliability: "very_low", warning: "Napaka modela za to skupino nepremičnin je previsoka za zanesljivo podporo odločitvi."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reliability, warnings := valuationAssessment(test.medianAPE)
			if reliability != test.reliability {
				t.Fatalf("reliability=%q want=%q", reliability, test.reliability)
			}
			if warnings[0] != "Statistična ocena ni uradna cenitev." ||
				warnings[1] != "Razpon je empiričen in ne vključuje vseh posebnosti posamezne nepremičnine." {
				t.Fatalf("base warnings=%v", warnings)
			}
			if test.warning != "" && warnings[len(warnings)-1] != test.warning {
				t.Fatalf("last warning=%q want=%q", warnings[len(warnings)-1], test.warning)
			}
		})
	}
}

func floatPointer(value float64) *float64 { return &value }
