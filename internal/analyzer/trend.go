package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cs2demo/platform/internal/domain"
	"github.com/cs2demo/platform/internal/prokb"
	"github.com/cs2demo/platform/internal/storage"
)

// BuildTrend 从 storage 拉的 trend rows 里筛出 player 的所有场次，聚合趋势。
func BuildTrend(playerName string, rows []storage.TrendRow, kb prokb.KB) domain.PlayerTrend {
	tr := domain.PlayerTrend{PlayerName: playerName}
	if playerName == "" || len(rows) == 0 {
		return tr
	}
	low := strings.ToLower(strings.TrimSpace(playerName))

	type record struct {
		row  storage.TrendRow
		role string
	}
	var matched []record
	for _, r := range rows {
		if r.Stats.Target.Name == "" {
			continue
		}
		if !strings.EqualFold(r.Stats.Target.Name, playerName) &&
			!strings.Contains(strings.ToLower(r.Stats.Target.Name), low) {
			continue
		}
		matched = append(matched, record{row: r, role: kb.RoleHints(r.Stats.Target)})
	}
	if len(matched) == 0 {
		return tr
	}
	// 按时间正序排，趋势走向才能判断
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].row.CreatedAt.Before(matched[j].row.CreatedAt)
	})

	tr.MatchesCount = len(matched)
	roleCount := map[string]int{}
	mapAgg := map[string]*domain.MapTrendStat{}
	totalADR, totalKAST, totalHS, totalKD := 0.0, 0.0, 0.0, 0.0
	wins := 0
	bestADR, worstADR := -1.0, 1e9
	var bestMap, worstMap string

	for _, m := range matched {
		st := m.row.Stats
		t := st.Target
		won := false
		switch t.Team {
		case "T":
			won = st.ScoreT > st.ScoreCT
		case "CT":
			won = st.ScoreCT > st.ScoreT
		}
		if won {
			wins++
		}
		kd := 0.0
		if t.Deaths > 0 {
			kd = float64(t.Kills) / float64(t.Deaths)
		}
		tp := domain.TrendPoint{
			DemoID:   m.row.DemoID,
			PlayedAt: m.row.CreatedAt,
			Map:      st.Map,
			Score:    fmt.Sprintf("T%d-%dCT", st.ScoreT, st.ScoreCT),
			Won:      won,
			Kills:    t.Kills,
			Deaths:   t.Deaths,
			ADR:      t.ADR,
			KAST:     t.KAST,
			HSPct:    t.HeadshotPct,
			Role:     m.role,
		}
		tr.Matches = append(tr.Matches, tp)
		totalADR += t.ADR
		totalKAST += t.KAST
		totalHS += t.HeadshotPct
		totalKD += kd
		roleCount[m.role]++

		mp := mapAgg[st.Map]
		if mp == nil {
			mp = &domain.MapTrendStat{Map: st.Map}
			mapAgg[st.Map] = mp
		}
		mp.Played++
		mp.AvgADR += t.ADR
		mp.AvgKAST += t.KAST
		if won {
			mp.WinRate += 1
		}

		if t.ADR > bestADR {
			bestADR = t.ADR
			bestMap = st.Map
		}
		if t.ADR < worstADR && t.ADR > 0 {
			worstADR = t.ADR
			worstMap = st.Map
		}
	}

	n := float64(tr.MatchesCount)
	tr.AvgADR = round2(totalADR / n)
	tr.AvgKAST = round2(totalKAST / n)
	tr.AvgHSPct = round2(totalHS / n)
	tr.AvgKD = round2(totalKD / n)
	tr.WinRate = round2(float64(wins) / n * 100)

	// stable role / role swings
	stable := ""
	maxC := 0
	for r, c := range roleCount {
		if c > maxC {
			stable = r
			maxC = c
		}
	}
	tr.StableRole = stable
	tr.RoleSwings = len(roleCount)
	tr.BestMap = bestMap
	tr.WorstMap = worstMap

	for _, mp := range mapAgg {
		if mp.Played > 0 {
			mp.AvgADR = round2(mp.AvgADR / float64(mp.Played))
			mp.AvgKAST = round2(mp.AvgKAST / float64(mp.Played))
			mp.WinRate = round2(mp.WinRate / float64(mp.Played) * 100)
		}
		tr.MapStats = append(tr.MapStats, *mp)
	}
	sort.Slice(tr.MapStats, func(i, j int) bool {
		return tr.MapStats[i].Played > tr.MapStats[j].Played
	})

	tr.ADRTrendDir = adrTrendDirection(tr.Matches)
	tr.Verdict = trendVerdict(tr)
	return tr
}

func adrTrendDirection(pts []domain.TrendPoint) string {
	n := len(pts)
	if n < 3 {
		return "样本不足"
	}
	half := n / 2
	earlySum, lateSum := 0.0, 0.0
	for i := 0; i < half; i++ {
		earlySum += pts[i].ADR
	}
	for i := half; i < n; i++ {
		lateSum += pts[i].ADR
	}
	early := earlySum / float64(half)
	late := lateSum / float64(n-half)
	delta := late - early
	switch {
	case delta >= 5:
		return fmt.Sprintf("上升(早期均值%.0f→近期%.0f)", early, late)
	case delta <= -5:
		return fmt.Sprintf("下滑(早期均值%.0f→近期%.0f)", early, late)
	default:
		return fmt.Sprintf("平稳(早期%.0f≈近期%.0f)", early, late)
	}
}

func trendVerdict(tr domain.PlayerTrend) string {
	if tr.MatchesCount < 3 {
		return fmt.Sprintf("仅 %d 场样本，建议至少积累 5 场再做趋势判断", tr.MatchesCount)
	}
	parts := []string{}
	parts = append(parts, fmt.Sprintf("近 %d 场 平均 ADR=%.0f / KAST=%.0f%% / HS%%=%.0f / 胜率=%.0f%%",
		tr.MatchesCount, tr.AvgADR, tr.AvgKAST, tr.AvgHSPct, tr.WinRate))
	parts = append(parts, "ADR 走势："+tr.ADRTrendDir)
	if tr.RoleSwings >= 3 {
		parts = append(parts, fmt.Sprintf("角色摇摆 %d 种（%s 主导），建议固化定位", tr.RoleSwings, tr.StableRole))
	} else if tr.StableRole != "" {
		parts = append(parts, fmt.Sprintf("稳定定位为 %s", tr.StableRole))
	}
	if tr.BestMap != "" && tr.WorstMap != "" && tr.BestMap != tr.WorstMap {
		parts = append(parts, fmt.Sprintf("强势地图：%s；薄弱地图：%s", tr.BestMap, tr.WorstMap))
	}
	return strings.Join(parts, "；")
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
