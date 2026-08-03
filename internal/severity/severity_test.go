package severity

import "testing"

func TestFromContextReturnsCritical_whenArticleConfirmsZeroDayExploitation(t *testing.T) {
	// Given an article that explicitly confirms a zero-day attack.
	zeroDay := true

	// When contextual severity is calculated.
	got := FromContext(0, zeroDay)

	// Then zero-day exploitation is critical even without a victim count.
	if got != Critical {
		t.Fatalf("severity = %q, want %q", got, Critical)
	}
}

func TestFromContextUsesVictimCountThresholds_whenVictimsAreExplicit(t *testing.T) {
	tests := []struct {
		name    string
		victims int
		want    Level
	}{
		{name: "mass impact", victims: 100_000, want: Critical},
		{name: "large impact", victims: 1_000, want: High},
		{name: "confirmed victims", victims: 1, want: Medium},
		{name: "not stated", victims: 0, want: Unknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When contextual severity is calculated from an explicit victim count.
			got := FromContext(test.victims, false)

			// Then the documented impact threshold is applied.
			if got != test.want {
				t.Fatalf("severity = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMaxKeepsHigherSignal_whenCVSSAndContextDiffer(t *testing.T) {
	// Given a medium CVSS result and critical contextual impact.
	cvss := FromCVSS(5.5)
	contextual := FromContext(500_000, false)

	// When both signals are combined.
	got := Max(cvss, contextual)

	// Then the higher severity is retained.
	if got != Critical {
		t.Fatalf("severity = %q, want %q", got, Critical)
	}
}
