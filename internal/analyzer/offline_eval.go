package analyzer

import (
	"fmt"
	"strings"

	"github.com/cs2demo/platform/internal/domain"
)

// offlineAimEval 枪法画像：距离分布 + 多杀 + 首杀对枪 + HS 分布。
func offlineAimEval(a *domain.AimQuality) string {
	if a == nil {
		return "枪法数据样本不足，建议下一场再观察"
	}
	parts := []string{}
	total := a.CloseKills + a.MidKills + a.LongKills
	if total == 0 {
		return "本场无击杀样本，无法评估枪法"
	}
	parts = append(parts, fmt.Sprintf("距离分布：近(%d)/中(%d)/远(%d)，平均 %.1fm",
		a.CloseKills, a.MidKills, a.LongKills, a.AvgKillDistance))

	switch {
	case a.LongKills >= 3 && a.HSLongRate >= 30:
		parts = append(parts, fmt.Sprintf("远距离爆头率 %.0f%%（>%d杀）属优秀级别", a.HSLongRate, a.LongKills))
	case a.CloseKills >= 5 && a.HSCloseRate < 30:
		parts = append(parts, fmt.Sprintf("近距离 HS%% 仅 %.0f%%，建议练定点高度", a.HSCloseRate))
	case a.HSCloseRate >= 50:
		parts = append(parts, fmt.Sprintf("近距离爆头率 %.0f%%，准星高度位置稳定", a.HSCloseRate))
	}

	if a.MultiKillRounds > 0 {
		parts = append(parts, fmt.Sprintf("%d 个回合打出 2+ 连杀", a.MultiKillRounds))
	}
	if a.FastestDoubleSec > 0 {
		switch {
		case a.FastestDoubleSec < 1.0:
			parts = append(parts, fmt.Sprintf("最快连杀 %.2fs，反应/转点拉满", a.FastestDoubleSec))
		case a.FastestDoubleSec < 2.5:
			parts = append(parts, fmt.Sprintf("最快连杀 %.2fs，节奏在线", a.FastestDoubleSec))
		default:
			parts = append(parts, fmt.Sprintf("最快连杀 %.2fs，转点偏慢可加练", a.FastestDoubleSec))
		}
	}
	if a.OpeningDuelWins+a.OpeningDuelLoss > 0 {
		parts = append(parts, fmt.Sprintf("首杀对枪 %d胜/%d负", a.OpeningDuelWins, a.OpeningDuelLoss))
	}
	return strings.Join(parts, "；")
}

// offlinePressureEval 高压 vs 普通对照，看心态稳定度。
func offlinePressureEval(p *domain.PressureSplit) string {
	if p == nil || (p.HighStakeRounds == 0 && p.NormalRounds == 0) {
		return "高压回合样本不足"
	}
	parts := []string{}
	parts = append(parts, fmt.Sprintf("高压回合 %d 个（残局/经济/赛点/连败3+）vs 普通回合 %d 个",
		p.HighStakeRounds, p.NormalRounds))
	if p.HighStakeRounds > 0 && p.NormalRounds > 0 {
		diff := p.HighStakeKAST - p.NormalKAST
		switch {
		case diff >= 5:
			parts = append(parts, fmt.Sprintf("高压 KAST %.0f%% > 普通 %.0f%%，逆境表现更稳", p.HighStakeKAST, p.NormalKAST))
		case diff <= -10:
			parts = append(parts, fmt.Sprintf("高压 KAST %.0f%% << 普通 %.0f%%，心态吃紧需要复盘", p.HighStakeKAST, p.NormalKAST))
		default:
			parts = append(parts, fmt.Sprintf("高压 KAST %.0f%% 与普通 %.0f%% 相近，发挥稳定", p.HighStakeKAST, p.NormalKAST))
		}
	}
	if p.ClutchPlayed > 0 {
		parts = append(parts, fmt.Sprintf("打过 %d 次残局，赢 %d 次", p.ClutchPlayed, p.ClutchWon))
	}
	if p.LosingStreakMax >= 3 {
		parts = append(parts, fmt.Sprintf("最大连败 %d 局，注意调整节奏避免心态崩盘", p.LosingStreakMax))
	}
	if p.EcoRoundKills > 0 {
		parts = append(parts, fmt.Sprintf("经济局贡献 %d 杀，对位枪表现到位", p.EcoRoundKills))
	}
	if p.MatchPointRounds > 0 {
		parts = append(parts, fmt.Sprintf("赛点回合 %d 个，关键分专注度需要保持", p.MatchPointRounds))
	}
	return strings.Join(parts, "；")
}

// offlineTeamSyncEval 团队配合：trade 率、跟进、孤立死。
func offlineTeamSyncEval(t *domain.TeamSync) string {
	if t == nil || (t.TradeKills == 0 && t.SoloDeaths == 0 && t.FollowupKills == 0) {
		return "团队配合样本不足"
	}
	parts := []string{}
	parts = append(parts, fmt.Sprintf("trade 击杀 %d / trade 死亡 %d（trade 率 %.0f%%）",
		t.TradeKills, t.TradeDeaths, t.TradeRate))
	if t.AvgTradeGapSec > 0 {
		switch {
		case t.AvgTradeGapSec <= 1.5:
			parts = append(parts, fmt.Sprintf("平均 trade 间隔 %.2fs，节奏紧凑", t.AvgTradeGapSec))
		case t.AvgTradeGapSec <= 3.0:
			parts = append(parts, fmt.Sprintf("平均 trade 间隔 %.2fs，可接受", t.AvgTradeGapSec))
		default:
			parts = append(parts, fmt.Sprintf("平均 trade 间隔 %.2fs 偏慢，跟进意识需加强", t.AvgTradeGapSec))
		}
	}
	if t.FollowupKills > 0 {
		parts = append(parts, fmt.Sprintf("跟进/交叉火力 %d 次", t.FollowupKills))
	}
	if t.SoloDeaths > 0 {
		switch {
		case t.SoloDeaths >= 5:
			parts = append(parts, fmt.Sprintf("孤立阵亡 %d 次，过于单干，应跟队友拉一线", t.SoloDeaths))
		case t.SoloDeaths >= 3:
			parts = append(parts, fmt.Sprintf("孤立阵亡 %d 次，注意贴队友站位", t.SoloDeaths))
		}
	}
	if t.BestTradePartner != "" && t.BestPartnerCount > 0 {
		parts = append(parts, fmt.Sprintf("最佳搭档 %s（%d 次同回合配合）", t.BestTradePartner, t.BestPartnerCount))
	}
	if t.StackedDeathRounds > 0 {
		parts = append(parts, fmt.Sprintf("有 %d 个回合全队 3+ 人死同点，team execute 失败模式集中", t.StackedDeathRounds))
	}
	return strings.Join(parts, "；")
}

// offlineSmokeEval 烟雾覆盖率：基于地图职业封烟点匹配。
func offlineSmokeEval(s *domain.SmokeReport) string {
	if s == nil || s.TotalSmokes == 0 {
		return "本场未投出烟雾弹，烟雾使用为 0"
	}
	parts := []string{}
	parts = append(parts, fmt.Sprintf("总烟雾 %d 颗，命中关键封烟点 %d 颗（准确率 %.0f%%）",
		s.TotalSmokes, s.AccurateSmokes, s.AccuracyPct))
	switch {
	case s.AccuracyPct >= 70:
		parts = append(parts, "封烟体系熟练")
	case s.AccuracyPct >= 40:
		parts = append(parts, "封烟基本功合格但 lineup 不稳定")
	default:
		parts = append(parts, "封烟落点散，建议练 lineup")
	}
	if s.KeyChokeMissed >= 2 {
		parts = append(parts, fmt.Sprintf("漏掉 %d 个关键控点没封住", s.KeyChokeMissed))
	}
	return strings.Join(parts, "；")
}

// offlineMovementEval 移动纪律：跑动开枪、孤立死、过度突进。
func offlineMovementEval(m *domain.MovementDiscipline) string {
	if m == nil || m.TotalKills == 0 {
		return "移动纪律样本不足"
	}
	parts := []string{}
	switch {
	case m.MovingKillRate >= 30:
		parts = append(parts, fmt.Sprintf("跑动开枪率 %.0f%% 偏高（>30%%），步枪精度受损", m.MovingKillRate))
	case m.MovingKillRate >= 15:
		parts = append(parts, fmt.Sprintf("跑动开枪率 %.0f%%，急停意识中等", m.MovingKillRate))
	default:
		parts = append(parts, fmt.Sprintf("跑动开枪率 %.0f%%，急停射击纪律到位", m.MovingKillRate))
	}
	if m.OvercommitDeaths >= 2 {
		parts = append(parts, fmt.Sprintf("早期突进死 %d 次（脱离全队主攻方向）", m.OvercommitDeaths))
	}
	if m.IsolationDeaths >= 2 {
		parts = append(parts, fmt.Sprintf("作为队伍首死 %d 次，开局曝光过早", m.IsolationDeaths))
	}
	if m.NoScopeKills > 0 {
		parts = append(parts, fmt.Sprintf("盲狙击杀 %d", m.NoScopeKills))
	}
	if m.JumpShotKills > 0 {
		parts = append(parts, fmt.Sprintf("跳投/极速移动击杀 %d", m.JumpShotKills))
	}
	return strings.Join(parts, "；")
}
