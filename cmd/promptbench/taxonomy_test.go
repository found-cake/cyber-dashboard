package main

import (
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/internal/summary"
)

func TestPromptListsEveryTaxonomyLabel(t *testing.T) {
	// Given the analysis prompt, which spells the allowed labels out in prose.
	prompt := summary.AnalyzeArticleSystemPrompt("en")

	// When each label the code validates against is looked up in that prose.
	for _, label := range append(append([]string{}, attackMethodLabels...), targetSectorLabels...) {
		// Then it is present: a label that exists in only one of the two places would either
		// be rejected after the model correctly returned it, or accepted while unlisted.
		if !strings.Contains(prompt, `"`+label+`"`) {
			t.Errorf("prompt does not offer taxonomy label %q", label)
		}
	}
}

func TestPromptOffersEveryPatchState(t *testing.T) {
	// Given the analysis prompt and the patch states the bench measures against.
	prompt := summary.AnalyzeArticleSystemPrompt("en")

	for _, state := range patchStates {
		// Then each one is spelled out in the prompt: a state the code knows but the
		// prompt never offers would be counted as coverage the model cannot deliver.
		if !strings.Contains(prompt, `"`+state+`"`) {
			t.Errorf("prompt does not offer patch state %q", state)
		}
	}
}

func TestFlawMethodsAreRealTaxonomyLabels(t *testing.T) {
	// Given the labels used to decide where a patch state is expected.
	for _, label := range flawMethods {
		// Then each is a label the prompt can actually return, so the denominator of the
		// patch-state measurement cannot silently become zero after a taxonomy rename.
		if !isAttackMethod(label) {
			t.Errorf("flaw method %q is not in the attack_method taxonomy", label)
		}
	}
}

func TestIncidentMethodSeparatesNonIncidentLabels(t *testing.T) {
	// Given every allowed attack_method label.
	for _, label := range attackMethodLabels {
		nonIncident := false
		for _, candidate := range nonIncidentLabels {
			if candidate == label {
				nonIncident = true
			}
		}

		// When the label is asked whether it describes a real attack.
		// Then the three non-incident labels answer no and every other label answers yes,
		// which is what forces threat_actor to "None" for articles with no attacker.
		if got := isIncidentMethod(label); got == nonIncident {
			t.Errorf("isIncidentMethod(%q) = %v, but non-incident is %v", label, got, nonIncident)
		}
	}
	if isIncidentMethod("SQL 인젝션") {
		t.Error("a label outside the taxonomy must not count as an incident")
	}
}

func TestActorFormClassification(t *testing.T) {
	// Given one actor string per form the prompt can produce.
	tests := []struct {
		name                       string
		actor                      string
		unidentified, languageOnly bool
	}{
		{name: "attributed group", actor: "Lazarus Group"},
		{name: "country linked", actor: "Unidentified Russia-linked actor", unidentified: true},
		{name: "language community", actor: "Unknown (Chinese-speaking)", languageOnly: true},
		{name: "AI operated", actor: aiOperatedActor},
		{name: "unattributed", actor: unknownActor},
		{name: "no attack", actor: noAttack},
		{name: "empty", actor: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When each form is classified.
			// Then only the corresponding placeholder recognizer matches.
			if got := isUnidentifiedActor(test.actor); got != test.unidentified {
				t.Errorf("isUnidentifiedActor(%q) = %v, want %v", test.actor, got, test.unidentified)
			}
			if got := isLanguageOnlyActor(test.actor); got != test.languageOnly {
				t.Errorf("isLanguageOnlyActor(%q) = %v, want %v", test.actor, got, test.languageOnly)
			}
		})
	}
}
