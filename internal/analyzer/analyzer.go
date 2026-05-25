package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cs2demo/platform/internal/domain"
	"github.com/cs2demo/platform/internal/prokb"
)

type Analyzer struct {
	llm LLMClient
	kb  prokb.KB
}

func New(provider, apiKey, baseURL, model string, kb prokb.KB) *Analyzer {
	return &Analyzer{llm: BuildLLMClient(provider, apiKey, baseURL, model), kb: kb}
}

func (a *Analyzer) ProviderName() string {
	if a.llm == nil {
		return "offline"
	}
	return a.llm.Name()
}

func DumpPrompt(stats domain.MatchStats, kb prokb.KB) (system, user string) {
	role := kb.RoleHints(stats.Target)
	baseline := kb.Lookup(stats.Map, role)
	cmp := buildComparison(stats.Target, baseline)
	return systemPrompt, buildPrompt(stats, baseline, cmp)
}

func (a *Analyzer) Analyze(ctx context.Context, demoID string, stats domain.MatchStats) (domain.AnalysisReport, error) {
	role := a.kb.RoleHints(stats.Target)
	baseline := a.kb.Lookup(stats.Map, role)
	comparison := buildComparison(stats.Target, baseline)

	if a.llm == nil {
		report := offlineReport(demoID, stats, baseline, comparison)
		normalizeTerms(&report)
		return report, nil
	}

	prompt := buildPrompt(stats, baseline, comparison)
	raw, err := a.llm.Complete(ctx, systemPrompt, prompt)
	if err != nil {
		report := offlineReport(demoID, stats, baseline, comparison)
		report.Verdict = "[LLM 调用失败，回退离线规则] " + report.Verdict
		normalizeTerms(&report)
		return report, fmt.Errorf("llm call: %w", err)
	}

	report, err := parseLLMReport(raw)
	if err != nil {
		report := offlineReport(demoID, stats, baseline, comparison)
		report.Verdict = "[LLM 输出无法解析，回退离线规则] " + report.Verdict
		normalizeTerms(&report)
		return report, fmt.Errorf("parse llm json: %w (raw_head=%s ... raw_tail=%s)", err, truncate(raw, 200), tail(raw, 200))
	}

	report.DemoID = demoID
	report.GeneratedAt = time.Now().UTC()
	report.Comparison = comparison
	if report.ProReference == "" {
		report.ProReference = baseline.Notes
	}
	mergeOfflineForMissing(&report, stats, baseline)
	normalizeTerms(&report)
	return report, nil
}

func mergeOfflineForMissing(r *domain.AnalysisReport, stats domain.MatchStats, baseline domain.ProBaseline) {
	off := offlineReport(r.DemoID, stats, baseline, r.Comparison)
	patched := []string{}
	if len(r.Strengths) == 0 {
		r.Strengths = off.Strengths
		patched = append(patched, "亮点")
	}
	if len(r.Weaknesses) == 0 {
		r.Weaknesses = off.Weaknesses
		patched = append(patched, "短板")
	}
	if len(r.Suggestions) == 0 {
		r.Suggestions = off.Suggestions
		patched = append(patched, "建议")
	}
	if len(r.RoundAnalyses) == 0 {
		r.RoundAnalyses = off.RoundAnalyses
		patched = append(patched, "回合分析")
	}
	if r.Verdict == "" {
		r.Verdict = off.Verdict
	}
	if r.OverallScore == 0 {
		r.OverallScore = off.OverallScore
	}
	if len(patched) > 0 {
		r.Verdict = "[LLM 输出截断, " + strings.Join(patched, "/") + " 由规则补齐] " + r.Verdict
	}
}

var termReplacements = []struct{ from, to string }{
	{"pistol局", "手枪局"},
	{"pistol 局", "手枪局"},
	{"Pistol局", "手枪局"},
	{"PISTOL局", "手枪局"},
	{"pistol round", "手枪局"},
	{"pistol-round", "手枪局"},
	{"force-buy", "强起局"},
	{"force buy", "强起局"},
	{"forcebuy", "强起局"},
	{"force局", "强起局"},
	{"Force局", "强起局"},
	{"force 局", "强起局"},
	{"semi-buy", "半起局"},
	{"semi buy", "半起局"},
	{"semibuy", "半起局"},
	{"semi局", "半起局"},
	{"Semi局", "半起局"},
	{"full-buy", "全装局"},
	{"full buy", "全装局"},
	{"fullbuy", "全装局"},
	{"full局", "全装局"},
	{"Full局", "全装局"},
	{"eco局", "经济局"},
	{"Eco局", "经济局"},
	{"ECO局", "经济局"},
	{"eco 局", "经济局"},
	{"eco round", "经济局"},
	{"clutch局", "残局"},
	{"clutch 局", "残局"},
	{"Clutch", "残局"},
	{"CLUTCH", "残局"},
}

var wordBoundaryReplacements = []struct{ from, to string }{
	{"pistol", "手枪局"},
	{"Pistol", "手枪局"},
	{"PISTOL", "手枪局"},
	{"force", "强起局"},
	{"Force", "强起局"},
	{"FORCE", "强起局"},
	{"semi", "半起局"},
	{"Semi", "半起局"},
	{"SEMI", "半起局"},
	{"eco", "经济局"},
	{"Eco", "经济局"},
	{"ECO", "经济局"},
	{"clutch", "残局"},
}

func cleanTerms(s string) string {
	if s == "" {
		return s
	}
	for _, r := range termReplacements {
		s = strings.ReplaceAll(s, r.from, r.to)
	}
	for _, r := range wordBoundaryReplacements {
		s = replaceWordBoundary(s, r.from, r.to)
	}
	return s
}

func replaceWordBoundary(s, from, to string) string {
	if !strings.Contains(s, from) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		idx := strings.Index(s[i:], from)
		if idx < 0 {
			b.WriteString(s[i:])
			break
		}
		abs := i + idx
		b.WriteString(s[i:abs])
		prevOK := abs == 0 || !isWordChar(s[abs-1])
		end := abs + len(from)
		nextOK := end >= len(s) || !isWordChar(s[end])
		if prevOK && nextOK {
			b.WriteString(to)
		} else {
			b.WriteString(from)
		}
		i = end
	}
	return b.String()
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

func normalizeTerms(r *domain.AnalysisReport) {
	r.Verdict = cleanTerms(r.Verdict)
	r.ProReference = cleanTerms(r.ProReference)
	for i := range r.Strengths {
		r.Strengths[i].Title = cleanTerms(r.Strengths[i].Title)
		r.Strengths[i].Detail = cleanTerms(r.Strengths[i].Detail)
	}
	for i := range r.Weaknesses {
		r.Weaknesses[i].Title = cleanTerms(r.Weaknesses[i].Title)
		r.Weaknesses[i].Detail = cleanTerms(r.Weaknesses[i].Detail)
	}
	for i := range r.Suggestions {
		r.Suggestions[i].Title = cleanTerms(r.Suggestions[i].Title)
		r.Suggestions[i].Detail = cleanTerms(r.Suggestions[i].Detail)
	}
	for i := range r.RoundAnalyses {
		r.RoundAnalyses[i].Tactic = cleanTerms(r.RoundAnalyses[i].Tactic)
		r.RoundAnalyses[i].Mistake = cleanTerms(r.RoundAnalyses[i].Mistake)
		r.RoundAnalyses[i].Clutch = cleanTerms(r.RoundAnalyses[i].Clutch)
	}
}

const systemPrompt = `你是一名资深的 CS2 教练，给中高级玩家做录像复盘。
输出必须是严格的 JSON 对象，符合下面 schema，不要多余文字、不要 markdown 包裹：
{
  "overall_score": <0-100 的整数>,
  "verdict":      "<一句话总结这场表现>",
  "strengths":    [{"title":"...","detail":"...","round":<可选整数>}],
  "weaknesses":   [{"title":"...","detail":"...","round":<可选整数>}],
  "suggestions":  [{"title":"...","detail":"..."}],
  "round_analyses": [
    {
      "round": <回合号>,
      "tactic": "<战术执行评注：先指出本回合全队战术意图（例如：T 方爆 A / 假 B 真 A / 中路控制 / 双 split），再说你在这个意图里扮演了什么角色、有没有跟上队友节奏。必须引用具体地图位置（A 大坑、B 门、香蕉道、A 小屋等）和队友动作（谁先死在哪、谁安弹、进攻方向）>",
      "mistake": "<具体失误：例如：队友打 A，你单走 B 道；爆点后你没跟进；CT 半场提前压点被秒>",
      "clutch": "<残局思路评注：站位、分散对手、打时间、利用炸弹时间等>",
      "verdict": "<执行/失误/亮眼/平淡 中选一个>"
    }
  ],
  "pro_reference":"<对比的职业选手参考语>"
}

【硬性规则 — 不遵守即不合格】
1. **本场地图是 prompt 里给定的那张**。所有建议、点位、训练方向**严禁**提到其它地图（dust2/mirage/inferno/ancient 等不能跨提）。如果给的是 ancient，就只能说 ancient 的位置（A 大房、B 坡、洞穴、连接处等）。
2. **战术术语全部用中文**。允许出现的局型只有这五个：手枪局、经济局、强起局、半起局、全装局。残局用"残局"或"1vN"，禁止 clutch。**输出里任何英文经济/局型术语视为不合格，会被自动剔除并降级为离线规则版**。武器名（USP-S/AK47 等）可保留英文。
3. **strengths/weaknesses 各 3-4 条，suggestions 3-4 条，detail 50 字以内**。
4. **round_analyses 覆盖 6-10 个关键回合**（首杀/末杀/残局/连续阵亡/经济翻盘/全队战术失败回合），不逐回合列。
5. **tactic 字段必须包含三要素**：① 全队战术意图（推测：例如"T 方打 A 大坑爆点"）② 你的实际动作 vs 战术意图（执行/脱节/单干）③ 引用至少一个具体位置名 + 至少一个队友名字。
6. clutch 字段仅当 clutch_situation 非空时才输出。
7. 用第二人称"你"，直接、专业、不绕弯。中文回答。`

func buildPrompt(stats domain.MatchStats, baseline domain.ProBaseline, cmp []domain.MetricCompare) string {
	var b strings.Builder
	t := stats.Target
	fmt.Fprintf(&b, "## 比赛信息\n地图: %s（请只针对这张地图给建议，不要跨提其它地图）\n", stats.Map)
	fmt.Fprintf(&b, "比分: T %d - %d CT, 总回合: %d, 时长: %ds\n\n",
		stats.ScoreT, stats.ScoreCT, stats.RoundsTotal, stats.DurationSec)

	fmt.Fprintf(&b, "## 目标玩家 %s [%s]\n", t.Name, t.Team)
	fmt.Fprintf(&b, "K/D/A: %d/%d/%d, HS%%: %.1f%%, ADR: %.1f, KAST: %.1f%%\n",
		t.Kills, t.Deaths, t.Assists, t.HeadshotPct, t.ADR, t.KAST)
	fmt.Fprintf(&b, "投掷物伤害: %d, 闪光助攻: %d\n", t.UtilityDamage, t.FlashAssists)

	if len(t.WeaponKills) > 0 {
		fmt.Fprintf(&b, "武器击杀分布: ")
		for w, n := range t.WeaponKills {
			fmt.Fprintf(&b, "%s=%d ", w, n)
		}
		b.WriteString("\n")
	}
	if len(t.GrenadeUsage) > 0 {
		fmt.Fprintf(&b, "投掷物使用: ")
		for g, n := range t.GrenadeUsage {
			fmt.Fprintf(&b, "%s=%d ", g, n)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "\n## 队友\n")
	for _, m := range stats.Teammates {
		fmt.Fprintf(&b, "- %s K/D=%d/%d ADR=%.0f KAST=%.0f%%\n", m.Name, m.Kills, m.Deaths, m.ADR, m.KAST)
	}
	fmt.Fprintf(&b, "\n## 对手\n")
	for _, m := range stats.Opponents {
		fmt.Fprintf(&b, "- %s K/D=%d/%d ADR=%.0f KAST=%.0f%%\n", m.Name, m.Kills, m.Deaths, m.ADR, m.KAST)
	}

	fmt.Fprintf(&b, "\n## 职业选手基线（%s · %s）\n", baseline.Role, baseline.Map)
	fmt.Fprintf(&b, "ADR: %.1f, KAST: %.1f%%, HS%%: %.1f%%, 投掷伤害: %d\n备注: %s\n",
		baseline.ADR, baseline.KAST, baseline.HeadshotPct, baseline.UtilityDamage, baseline.Notes)

	fmt.Fprintf(&b, "\n## 数据对比\n")
	for _, c := range cmp {
		fmt.Fprintf(&b, "- %s: 你 %.1f vs 职业 %.1f → %s\n", c.Metric, c.You, c.ProMedian, c.Verdict)
	}

	if len(stats.Rounds) > 0 {
		fmt.Fprintf(&b, "\n## 回合时间轴 + 战术上下文（共 %d 回合）\n", len(stats.Rounds))
		fmt.Fprintf(&b, "（下方经济列已全部中文化，你的输出必须沿用中文，不得再翻回英文）\n\n")
		for _, r := range stats.Rounds {
			parts := []string{fmt.Sprintf("R%d %s胜(%s)", r.Number, r.WinnerTeam, r.EndReason)}
			parts = append(parts, fmt.Sprintf("经济:你方=%s/装备$%d 对手=%s/装备$%d",
				econCN(r.TargetTeamEcon), r.TargetTeamEquipValue, econCN(r.OpponentTeamEcon), r.OpponentTeamEquipValue))
			if r.FirstContactSec > 0 {
				parts = append(parts, fmt.Sprintf("首接触%.0fs", r.FirstContactSec))
			}
			if r.TargetKills > 0 {
				parts = append(parts, fmt.Sprintf("你%dkill", r.TargetKills))
			}
			if r.TargetDied {
				zone := r.TargetDeathZone
				if zone == "" {
					zone = "未知点位"
				}
				parts = append(parts, fmt.Sprintf("你阵亡@%.0fs[%s]", r.TargetDeathTimeSec-r.StartTimeSec, zone))
			} else {
				parts = append(parts, "你存活")
			}
			if len(r.TeamPushZones) > 0 {
				parts = append(parts, "队友死位:["+strings.Join(r.TeamPushZones, ",")+"]")
			}
			if r.TeamFirstDeath != "" {
				parts = append(parts, "队首死:"+r.TeamFirstDeath)
			}
			if r.BombPlanted {
				bombInfo := "炸弹安放"
				if r.BombSiteZone != "" {
					bombInfo += "@" + r.BombSiteZone
				}
				if r.BombDefused {
					bombInfo += "→拆除"
				}
				parts = append(parts, bombInfo)
			}
			if r.ClutchSituation != "" {
				parts = append(parts, "残局"+r.ClutchSituation+"="+r.ClutchOutcome)
			}
			if len(r.TargetEvents) > 0 {
				ws := []string{}
				for _, e := range r.TargetEvents {
					tag := e.Weapon
					if e.Headshot {
						tag += "(HS)"
					}
					if e.VictimZone != "" {
						tag += "@" + e.VictimZone
					}
					ws = append(ws, tag)
				}
				parts = append(parts, "你的击杀:["+strings.Join(ws, ",")+"]")
			}
			fmt.Fprintln(&b, strings.Join(parts, " | "))
		}
	}

	b.WriteString("\n请基于以上数据生成结构化点评 JSON。重点：每条 round_analyses 必须从全队战术意图（推测）讲起，对照你的实际动作，引用具体地图位置和队友名字。所有建议只能针对当前地图，禁止跨地图举例。")
	return b.String()
}

func econCN(e string) string {
	switch e {
	case "pistol":
		return "手枪局"
	case "eco":
		return "经济局"
	case "force":
		return "强起局"
	case "semi":
		return "半起局"
	case "full":
		return "全装局"
	}
	return e
}

func buildComparison(p domain.PlayerStats, b domain.ProBaseline) []domain.MetricCompare {
	mk := func(name string, you, pro float64) domain.MetricCompare {
		v := "持平"
		ratio := 0.0
		if pro > 0 {
			ratio = (you - pro) / pro
		}
		switch {
		case ratio >= 0.1:
			v = "优于职业基线"
		case ratio <= -0.15:
			v = "差距明显"
		case ratio < 0:
			v = "略低于基线"
		}
		return domain.MetricCompare{Metric: name, You: you, ProMedian: pro, Verdict: v}
	}
	return []domain.MetricCompare{
		mk("ADR", p.ADR, b.ADR),
		mk("KAST%", p.KAST, b.KAST),
		mk("HS%", p.HeadshotPct, b.HeadshotPct),
		mk("UtilityDamage", float64(p.UtilityDamage), float64(b.UtilityDamage)),
	}
}

func parseLLMReport(raw string) (domain.AnalysisReport, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i > 0 {
		raw = raw[i:]
	}
	last := strings.LastIndex(raw, "}")
	if last >= 0 && last < len(raw)-1 {
		raw = raw[:last+1]
	}
	var r domain.AnalysisReport
	if err := json.Unmarshal([]byte(raw), &r); err == nil {
		return r, nil
	}
	repaired := repairTruncatedJSON(raw)
	if repaired != raw {
		var r2 domain.AnalysisReport
		if err := json.Unmarshal([]byte(repaired), &r2); err == nil {
			return r2, nil
		}
	}
	var empty domain.AnalysisReport
	return empty, json.Unmarshal([]byte(raw), &empty)
}

func repairTruncatedJSON(s string) string {
	if !strings.HasPrefix(strings.TrimSpace(s), "{") {
		return s
	}
	stack := []byte{}
	inStr := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inStr {
			escape = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 && !inStr {
		return s
	}
	out := s
	if inStr {
		out += `"`
	}
	for i := len(stack) - 1; i >= 0; i-- {
		out += string(stack[i])
	}
	return out
}

func offlineReport(demoID string, stats domain.MatchStats, baseline domain.ProBaseline, cmp []domain.MetricCompare) domain.AnalysisReport {
	t := stats.Target
	r := domain.AnalysisReport{
		DemoID:       demoID,
		GeneratedAt:  time.Now().UTC(),
		Comparison:   cmp,
		ProReference: baseline.Notes + "（离线模式：未调用 LLM，仅基于规则）",
	}

	kd := 0.0
	if t.Deaths > 0 {
		kd = float64(t.Kills) / float64(t.Deaths)
	}
	score := 50
	score += int((t.ADR - baseline.ADR) / 2)
	score += int((t.KAST - baseline.KAST) / 3)
	if kd > 1.2 {
		score += 5
	}
	if score < 10 {
		score = 10
	}
	if score > 95 {
		score = 95
	}
	r.OverallScore = score
	r.Verdict = fmt.Sprintf("本场 K/D=%.2f, ADR=%.1f, KAST=%.1f%%，相对职业 %s 基线整体 %s",
		kd, t.ADR, t.KAST, baseline.Role, overallVerdict(cmp))

	if t.HeadshotPct >= baseline.HeadshotPct {
		r.Strengths = append(r.Strengths, domain.ReportPoint{
			Title:  "枪法精度优秀",
			Detail: fmt.Sprintf("HS%% %.1f%% 达到甚至超过职业基线 %.1f%%，瞄准位置稳定", t.HeadshotPct, baseline.HeadshotPct),
		})
	}
	if t.KAST >= baseline.KAST {
		r.Strengths = append(r.Strengths, domain.ReportPoint{
			Title:  "回合参与度高",
			Detail: fmt.Sprintf("KAST %.1f%% 高于基线 %.1f%%，对团队贡献稳定", t.KAST, baseline.KAST),
		})
	}
	if t.UtilityDamage >= baseline.UtilityDamage {
		r.Strengths = append(r.Strengths, domain.ReportPoint{
			Title:  "投掷物利用合格",
			Detail: fmt.Sprintf("投掷伤害 %d 达到基线 %d", t.UtilityDamage, baseline.UtilityDamage),
		})
	}
	if len(r.Strengths) == 0 {
		r.Strengths = []domain.ReportPoint{{Title: "保持稳定", Detail: "无明显高光也无明显短板，处于成长期"}}
	}

	if t.ADR < baseline.ADR*0.85 {
		r.Weaknesses = append(r.Weaknesses, domain.ReportPoint{
			Title:  "ADR 偏低",
			Detail: fmt.Sprintf("ADR %.1f 显著低于基线 %.1f，输出贡献不足", t.ADR, baseline.ADR),
		})
	}
	if t.KAST < baseline.KAST*0.9 {
		r.Weaknesses = append(r.Weaknesses, domain.ReportPoint{
			Title:  "回合参与度不足",
			Detail: fmt.Sprintf("KAST %.1f%% 低于基线 %.1f%%，部分回合空白", t.KAST, baseline.KAST),
		})
	}
	if t.UtilityDamage < baseline.UtilityDamage/2 {
		r.Weaknesses = append(r.Weaknesses, domain.ReportPoint{
			Title:  "投掷物使用浪费",
			Detail: fmt.Sprintf("投掷伤害仅 %d，远低于基线 %d", t.UtilityDamage, baseline.UtilityDamage),
		})
	}
	if len(r.Weaknesses) == 0 {
		r.Weaknesses = []domain.ReportPoint{{Title: "无显著短板", Detail: "整体数据均衡，可冲击下一档"}}
	}

	r.Suggestions = []domain.ReportPoint{
		{Title: "针对 " + baseline.Role + " 角色训练", Detail: baseline.Notes},
		{Title: "复盘 " + stats.Map + " 地图位置", Detail: "对照职业选手在 " + stats.Map + " 上的站位与投掷物使用，禁止跨地图借鉴"},
	}

	r.RoundAnalyses = ruleBasedRoundAnalyses(stats)
	return r
}

func ruleBasedRoundAnalyses(stats domain.MatchStats) []domain.RoundAnalysisOut {
	out := []domain.RoundAnalysisOut{}
	for _, rd := range stats.Rounds {
		ra, ok := buildOfflineRoundAnalysis(stats, rd)
		if !ok {
			continue
		}
		out = append(out, ra)
	}
	return out
}

func buildOfflineRoundAnalysis(stats domain.MatchStats, rd domain.RoundSummary) (domain.RoundAnalysisOut, bool) {
	myEcon := econCN(rd.TargetTeamEcon)
	oppEcon := econCN(rd.OpponentTeamEcon)
	equipDiff := rd.TargetTeamEquipValue - rd.OpponentTeamEquipValue

	pushZones := strings.Join(stripZoneCount(rd.TeamPushZones), "、")
	primaryDir := primaryPushZone(rd.TeamPushZones)

	earlyDeath := rd.TargetDied && rd.TargetDeathTimeSec-rd.StartTimeSec < 25
	deathSec := 0.0
	if rd.TargetDied {
		deathSec = rd.TargetDeathTimeSec - rd.StartTimeSec
	}

	verdict := "平淡"
	switch {
	case rd.ClutchSituation != "" && strings.Contains(rd.ClutchOutcome, "你赢下"):
		verdict = "亮眼"
	case rd.ClutchSituation != "":
		verdict = "残局"
	case rd.TargetKills >= 3:
		verdict = "亮眼"
	case earlyDeath:
		verdict = "失误"
	case rd.TargetKills >= 1:
		verdict = "执行"
	}

	var tactic strings.Builder
	fmt.Fprintf(&tactic, "本回合你方为%s（装备$%d）, 对手为%s（装备$%d）", myEcon, rd.TargetTeamEquipValue, oppEcon, rd.OpponentTeamEquipValue)
	switch {
	case equipDiff <= -10000:
		tactic.WriteString("，火力差距悬殊，本回合定位为消耗或抢枪")
	case equipDiff <= -3000:
		tactic.WriteString("，装备处于劣势，正面硬刚不利")
	case equipDiff >= 10000:
		tactic.WriteString("，装备压制对手，应主动找会战")
	}
	tactic.WriteString("。")

	if primaryDir != "" {
		fmt.Fprintf(&tactic, "全队主攻方向看队友死位集中在%s，意图大概率是%s", pushZones, inferIntent(stats.Map, primaryDir))
		if rd.TeamFirstDeath != "" {
			fmt.Fprintf(&tactic, "（首死%s）", rd.TeamFirstDeath)
		}
		tactic.WriteString("；")
	} else if rd.TeamFirstDeath != "" {
		fmt.Fprintf(&tactic, "队伍首死在%s，", rd.TeamFirstDeath)
	}

	switch {
	case rd.TargetDied && rd.TargetDeathZone != "" && primaryDir != "" && !strings.Contains(rd.TargetDeathZone, primaryDir):
		fmt.Fprintf(&tactic, "你却阵亡于%s，与全队进攻方向脱节，建议下次跟上队友节奏", rd.TargetDeathZone)
	case rd.TargetDied && rd.TargetDeathZone != "":
		fmt.Fprintf(&tactic, "你%.0fs 阵亡于%s，时机和站位需要复盘", deathSec, rd.TargetDeathZone)
	case rd.TargetKills >= 2 && primaryDir != "":
		fmt.Fprintf(&tactic, "你打出 %d kill 配合队伍%s方向推进，执行到位", rd.TargetKills, primaryDir)
	case rd.TargetKills >= 1:
		fmt.Fprintf(&tactic, "你打出 %d kill 跟住团队节奏", rd.TargetKills)
	default:
		tactic.WriteString("你没有击杀贡献，建议加强卡点或投掷物配合")
	}
	tactic.WriteString("。")

	ra := domain.RoundAnalysisOut{
		Round:   rd.Number,
		Tactic:  tactic.String(),
		Verdict: verdict,
	}

	switch {
	case earlyDeath:
		zone := rd.TargetDeathZone
		if zone == "" {
			zone = "未知点位"
		}
		ra.Mistake = fmt.Sprintf("开局 %.0fs 抢点失败阵亡于%s，前期信息没拿到反而把人头送给对手", deathSec, zone)
	case rd.TargetDied && primaryDir != "" && rd.TargetDeathZone != "" && !strings.Contains(rd.TargetDeathZone, primaryDir):
		ra.Mistake = fmt.Sprintf("队伍主攻%s，你单走至%s阵亡，方向脱节导致以多打少机会丢失", primaryDir, rd.TargetDeathZone)
	case equipDiff >= 5000 && rd.TargetKills == 0 && rd.TargetDied:
		ra.Mistake = "装备占优却没拿下击杀，会战节奏没把握住"
	}

	if rd.ClutchSituation != "" {
		who := "你"
		if !strings.Contains(rd.ClutchOutcome, "你") {
			who = strings.SplitN(rd.ClutchOutcome, "赢", 2)[0]
			who = strings.SplitN(who, "输", 2)[0]
		}
		switch {
		case strings.Contains(rd.ClutchOutcome, "你赢下"):
			ra.Clutch = fmt.Sprintf("你%s拉满，思路正确：拆解多人优先打孤立点位，利用炸弹/时间分摊敌方注意力", rd.ClutchSituation)
		case strings.Contains(rd.ClutchOutcome, "你输掉"):
			ra.Clutch = fmt.Sprintf("你%s告负，复盘点：是否过早暴露位置/没用拐角分散对手", rd.ClutchSituation)
		default:
			ra.Clutch = fmt.Sprintf("%s打%s，结果：%s", who, rd.ClutchSituation, rd.ClutchOutcome)
		}
	}

	if ra.Tactic == "" && ra.Mistake == "" && ra.Clutch == "" && len(rd.TargetEvents) == 0 {
		return ra, false
	}
	return ra, true
}

func stripZoneCount(zs []string) []string {
	out := make([]string, 0, len(zs))
	for _, z := range zs {
		if i := strings.Index(z, "×"); i > 0 {
			out = append(out, z[:i])
		} else {
			out = append(out, z)
		}
	}
	return out
}

func primaryPushZone(zs []string) string {
	if len(zs) == 0 {
		return ""
	}
	z := zs[0]
	if i := strings.Index(z, "×"); i > 0 {
		z = z[:i]
	}
	return z
}

func inferIntent(mapName, zone string) string {
	z := strings.ToLower(zone)
	switch {
	case strings.Contains(z, "a") && (strings.Contains(z, "大") || strings.Contains(z, "长") || strings.Contains(z, "点")):
		return "打 A 包点 / 爆 A"
	case strings.Contains(z, "b") && (strings.Contains(z, "点") || strings.Contains(z, "门") || strings.Contains(z, "坡")):
		return "打 B 包点 / 爆 B"
	case strings.Contains(z, "中") || strings.Contains(z, "洞") || strings.Contains(z, "连接"):
		return "中路控制 / split 准备"
	case strings.Contains(z, "香蕉"):
		return "香蕉道压制 / B 进攻"
	}
	return "建立地图控制"
}

func overallVerdict(cmp []domain.MetricCompare) string {
	better, worse := 0, 0
	for _, c := range cmp {
		switch c.Verdict {
		case "优于职业基线":
			better++
		case "差距明显":
			worse += 2
		case "略低于基线":
			worse++
		}
	}
	if better > worse {
		return "表现优秀"
	}
	if worse > better+1 {
		return "存在明显差距"
	}
	return "处于及格线"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
