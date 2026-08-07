package main

import "strings"

// attackMethodLabels is the closed set of attack_method labels the analysis prompt allows.
// The dashboard groups by exact string, so any value outside this set becomes its own bar
// and fragments the distribution.
var attackMethodLabels = []string{
	"APT / Espionage",
	"Supply Chain",
	"Malware / Stealer",
	"Ransomware",
	"Botnet",
	"Financial / Crypto",
	"Social Engineering",
	"Vulnerability Exploitation",
	"AI / LLM Abuse",
	"Denial of Service",
	"Insider Threat",
	"Data Breach / Unauthorized Access",
	"Vulnerability Disclosure",
	"Industry / Guidance",
	"None",
}

// nonIncidentLabels are the attack_method labels that describe an article with no real
// attack in it. They exist because a single "None" bucket was the largest bar on the chart
// while merging a critical unpatched advisory with a vendor press release. Every one of them
// forces threat_actor to noAttack.
var nonIncidentLabels = []string{
	"Vulnerability Disclosure",
	"Industry / Guidance",
	"None",
}

// patchStates is the closed set of patch_available values the analysis prompt allows. The
// client folds anything it does not recognize into the empty value, so this list is not a
// filter over model output: it exists so the prompt and the code cannot drift apart on
// which three words are offered.
var patchStates = []string{"yes", "no", "unknown"}

// flawMethods are the attack_method labels whose articles are about a specific weakness,
// which is where a patch state is expected to be reported.
var flawMethods = []string{"Vulnerability Disclosure", "Vulnerability Exploitation"}

// isFlawMethod reports whether an article with this label should carry a patch state.
func isFlawMethod(label string) bool { return contains(flawMethods, label) }

// targetSectorLabels is the closed set of target_sector labels the analysis prompt allows.
var targetSectorLabels = []string{
	"Government",
	"Finance",
	"Technology",
	"Telecommunications",
	"Healthcare",
	"Education / Research",
	"Manufacturing",
	"Critical Infrastructure",
	"Retail / Consumer",
	"Media / Entertainment",
	"General",
}

// noAttack and unknownActor are the two sentinel values the prompt pins to English so a
// Korean run does not split them into separate bars.
const (
	noAttack     = "None"
	unknownActor = "Unknown"
	// aiOperatedActor is the fixed wording for an attack the article says an AI agent ran
	// itself. It is one string rather than a form with a slot in it, so every such attack
	// lands on one bar instead of on the name of whichever model the article happened to
	// mention. Attribution to a group or a country still wins over it.
	aiOperatedActor = "Unknown (AI-operated)"
)

// isAttackMethod reports whether label is one of the allowed attack_method values.
func isAttackMethod(label string) bool { return contains(attackMethodLabels, label) }

// isIncidentMethod reports whether the label describes a real attack. Callers pair it with
// threat_actor: a non-incident article has no attacker, so its actor must be noAttack.
func isIncidentMethod(label string) bool {
	return isAttackMethod(label) && !contains(nonIncidentLabels, label)
}

// isTargetSector reports whether label is one of the allowed target_sector values.
func isTargetSector(label string) bool { return contains(targetSectorLabels, label) }

// isUnidentifiedActor matches the "Unidentified {Country}-linked actor" form the prompt
// uses when an attack is tied to a state but no group is named.
func isUnidentifiedActor(actor string) bool {
	return strings.HasPrefix(actor, "Unidentified ") && strings.HasSuffix(actor, "-linked actor")
}

// isLanguageOnlyActor matches the "Unknown ({Language}-speaking)" form, which records a
// language community without claiming a nationality.
func isLanguageOnlyActor(actor string) bool {
	return strings.HasPrefix(actor, "Unknown (") && strings.HasSuffix(actor, "-speaking)")
}

// isNamedActor reports whether the actor is an attributed group rather than one of the
// sentinel or evidence-preserving placeholder forms.
func isNamedActor(actor string) bool {
	switch {
	case actor == "", actor == noAttack, actor == unknownActor, actor == aiOperatedActor:
		return false
	case isUnidentifiedActor(actor), isLanguageOnlyActor(actor):
		return false
	default:
		return true
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
