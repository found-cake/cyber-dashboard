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

func TestFromVulnerabilityAdjustsTheScore_forReachabilityAndPatchState(t *testing.T) {
	const remote = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
	tests := []struct {
		name           string
		score          float64
		vector         string
		patchAvailable string
		want           Level
	}{
		{name: "remote unauthenticated flaw", score: 9.8, vector: remote, want: Critical},
		{name: "same score but needs local access", score: 9.8, vector: "CVSS:3.1/AV:L/AC:L/PR:L/UI:N", want: High},
		{name: "same score but needs admin rights", score: 9.8, vector: "CVSS:3.1/AV:N/AC:L/PR:H/UI:N", want: High},
		{name: "CVSS v2 requiring authentication", score: 9.0, vector: "AV:N/AC:L/Au:S/C:C/I:C/A:C", want: High},
		// A released fix is the normal case in security writing, so treating it as a reason
		// to lower severity would quietly drain the top band. It is recorded, not scored.
		{name: "fix already released", score: 9.8, vector: remote, patchAvailable: PatchAvailable, want: Critical},
		{name: "no fix yet", score: 7.5, vector: remote, patchAvailable: PatchUnavailable, want: Critical},
		{name: "patch state not stated", score: 7.5, vector: remote, patchAvailable: "unknown", want: High},
		{name: "unreachable stays graded", score: 4.0, vector: "CVSS:3.1/AV:P/AC:H/PR:H/UI:R", patchAvailable: PatchAvailable, want: Low},
		{name: "no scored CVE", score: 0, vector: remote, patchAvailable: PatchUnavailable, want: Unknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When the CVE is graded from its score, vector, and patch state together.
			got := FromVulnerability(test.score, test.vector, test.patchAvailable)

			// Then reachability and an available fix move the score by one step each, and
			// neither can grade an article that has no scored CVE behind it.
			if got != test.want {
				t.Fatalf("severity = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFromDamageUsesLossThresholds_whenTheArticleStatesAnAmount(t *testing.T) {
	tests := []struct {
		name   string
		damage int64
		want   Level
	}{
		{name: "bridge drained", damage: 100_000_000, want: Critical},
		{name: "major theft", damage: 10_000_000, want: High},
		{name: "sizable loss", damage: 100_000, want: Medium},
		{name: "small loss", damage: 1, want: Low},
		{name: "not stated", damage: 0, want: Unknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When severity is calculated from the stated financial damage.
			got := FromDamage(test.damage)

			// Then the documented loss threshold is applied.
			if got != test.want {
				t.Fatalf("severity = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMaxUsesDamage_whenItIsTheOnlyImpactSignal(t *testing.T) {
	// Given a crypto theft article: no CVE, no victim count, only a stolen amount.
	cvss := FromCVSS(0)
	contextual := FromContext(0, false)

	// When every signal is combined.
	got := Max(cvss, contextual, FromDamage(190_000_000))

	// Then the loss alone carries the article instead of leaving it UNKNOWN.
	if got != Critical {
		t.Fatalf("severity = %q, want %q", got, Critical)
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
