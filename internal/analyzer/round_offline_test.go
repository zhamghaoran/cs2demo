package analyzer

import (
	"strings"
	"testing"

	"github.com/cs2demo/platform/internal/domain"
)

func TestBuildOfflineRoundAnalysis(t *testing.T) {
	stats := domain.MatchStats{Map: "de_ancient"}
	rd := domain.RoundSummary{
		Number: 1, StartTimeSec: 0, EndTimeSec: 200,
		TargetTeamEcon: "pistol", OpponentTeamEcon: "pistol",
		TargetTeamEquipValue: 1500, OpponentTeamEquipValue: 1400,
		TargetKills:     3,
		TargetDied:      true,
		TargetDeathTimeSec: 141,
		TargetDeathZone: "B点/B坡",
		TeamPushZones:   []string{"中路/洞穴×3", "B点/B坡×2"},
		TeamFirstDeath:  "alex666@A点/A大房",
		ClutchSituation: "1v3",
		ClutchOutcome:   "你赢下1v3",
	}
	ra, ok := buildOfflineRoundAnalysis(stats, rd)
	if !ok {
		t.Fatal("expected analysis built")
	}
	checks := []struct{ name, sub string }{
		{"has econ chinese", "手枪局"},
		{"has push zones", "中路/洞穴"},
		{"has first death", "alex666"},
		{"has clutch verdict", "1v3"},
		{"has map zone", "B点/B坡"},
	}
	full := ra.Tactic + " " + ra.Mistake + " " + ra.Clutch
	for _, c := range checks {
		if !strings.Contains(full, c.sub) {
			t.Errorf("%s: expected %q in %q", c.name, c.sub, full)
		}
	}
	if ra.Verdict != "亮眼" {
		t.Errorf("expected verdict=亮眼 for won clutch, got %q", ra.Verdict)
	}
	if strings.Contains(full, "pistol") || strings.Contains(full, "Clutch") || strings.Contains(full, "force") {
		t.Errorf("English term leaked: %q", full)
	}
}

func TestBuildOfflineRoundAnalysisEarlyDeath(t *testing.T) {
	stats := domain.MatchStats{Map: "de_ancient"}
	rd := domain.RoundSummary{
		Number: 5, StartTimeSec: 100, EndTimeSec: 250,
		TargetTeamEcon: "full", OpponentTeamEcon: "full",
		TargetTeamEquipValue: 22000, OpponentTeamEquipValue: 22000,
		TargetDied:         true,
		TargetDeathTimeSec: 110,
		TargetDeathZone:    "A坡道/连接处",
		TeamPushZones:      []string{"B点/B坡×4"},
	}
	ra, ok := buildOfflineRoundAnalysis(stats, rd)
	if !ok {
		t.Fatal("expected analysis built")
	}
	if ra.Verdict != "失误" {
		t.Errorf("expected verdict=失误, got %q", ra.Verdict)
	}
	if !strings.Contains(ra.Mistake, "A坡道") {
		t.Errorf("expected mistake to mention death zone, got %q", ra.Mistake)
	}
	if !strings.Contains(ra.Tactic, "B点") {
		t.Errorf("expected tactic to mention team push direction, got %q", ra.Tactic)
	}
	if strings.Contains(ra.Tactic, "full") || strings.Contains(ra.Mistake, "full") {
		t.Errorf("English term leaked: tactic=%q mistake=%q", ra.Tactic, ra.Mistake)
	}
}
