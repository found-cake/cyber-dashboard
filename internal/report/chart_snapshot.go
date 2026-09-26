package report

import (
	"encoding/json"
	"fmt"

	"github.com/found-cake/cyber-dashboard/api"
)

type chartSnapshot struct {
	AttackMethods []api.BreakdownRow `json:"attack_methods"`
	ThreatActors  []api.BreakdownRow `json:"threat_actors"`
}

func encodeChartSnapshot(charts *api.ReportCharts) (string, error) {
	if charts == nil {
		return "", nil
	}
	encoded, err := json.Marshal(chartSnapshot{AttackMethods: charts.AttackMethods, ThreatActors: charts.ThreatActors})
	if err != nil {
		return "", fmt.Errorf("encode report chart snapshot: %w", err)
	}
	return string(encoded), nil
}

func decodeChartSnapshot(value string) *api.ReportCharts {
	var snapshot *chartSnapshot
	if err := json.Unmarshal([]byte(value), &snapshot); err != nil || snapshot == nil {
		return nil
	}
	return &api.ReportCharts{AttackMethods: snapshot.AttackMethods, ThreatActors: snapshot.ThreatActors}
}
