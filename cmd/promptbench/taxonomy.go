package main

import "strings"

// attackMethodLabels is the prompt's closed set; other values fragment dashboard buckets.
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

// nonIncidentLabels describe no real attack and require a noAttack actor.
var nonIncidentLabels = []string{
	"Vulnerability Disclosure",
	"Industry / Guidance",
	"None",
}

// patchStates mirrors the prompt's closed set so prompt and code cannot drift.
var patchStates = []string{"yes", "no", "unknown"}

// flawMethods are labels for which a patch state is expected.
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

// English sentinels keep localized runs in the same dashboard buckets.
const (
	noAttack     = "None"
	unknownActor = "Unknown"
	// aiOperatedActor groups autonomous AI attacks unless stronger attribution exists.
	aiOperatedActor = "Unknown (AI-operated)"
)

// isAttackMethod reports whether label is one of the allowed attack_method values.
func isAttackMethod(label string) bool { return contains(attackMethodLabels, label) }

// isIncidentMethod reports whether the label describes a real attack with an actor.
func isIncidentMethod(label string) bool {
	return isAttackMethod(label) && !contains(nonIncidentLabels, label)
}

// isTargetSector reports whether label is one of the allowed target_sector values.
func isTargetSector(label string) bool { return contains(targetSectorLabels, label) }

// isUnidentifiedActor matches the prompt's unnamed state-linked actor form.
func isUnidentifiedActor(actor string) bool {
	return strings.HasPrefix(actor, "Unidentified ") && strings.HasSuffix(actor, "-linked actor")
}

// isLanguageOnlyActor matches language evidence that does not establish nationality.
func isLanguageOnlyActor(actor string) bool {
	return strings.HasPrefix(actor, "Unknown (") && strings.HasSuffix(actor, "-speaking)")
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
