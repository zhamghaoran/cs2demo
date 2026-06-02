package parser

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/cs2demo/platform/internal/domain"
)

// computeAimQuality 根据 target 击杀流水算枪法画像。
func computeAimQuality(target domain.PlayerStats, rounds []domain.RoundSummary, killTimes []float64) *domain.AimQuality {
	if len(target.Highlights) == 0 {
		return nil
	}
	aq := &domain.AimQuality{}
	hsClose, hsLong := 0, 0
	totalDist := 0.0
	for _, k := range target.Highlights {
		switch {
		case k.Distance < 8:
			aq.CloseKills++
			if k.Headshot {
				hsClose++
			}
		case k.Distance < 20:
			aq.MidKills++
		default:
			aq.LongKills++
			if k.Headshot {
				hsLong++
			}
		}
		totalDist += k.Distance
	}
	n := len(target.Highlights)
	if n > 0 {
		aq.AvgKillDistance = round2(totalDist / float64(n))
	}
	if aq.CloseKills > 0 {
		aq.HSCloseRate = round2(float64(hsClose) / float64(aq.CloseKills) * 100)
	}
	if aq.LongKills > 0 {
		aq.HSLongRate = round2(float64(hsLong) / float64(aq.LongKills) * 100)
	}

	// 多杀回合 + 最快连杀间隔
	for _, r := range rounds {
		if r.TargetKills >= 2 {
			aq.MultiKillRounds++
		}
	}
	if len(killTimes) >= 2 {
		sort.Float64s(killTimes)
		fastest := math.MaxFloat64
		for i := 1; i < len(killTimes); i++ {
			gap := killTimes[i] - killTimes[i-1]
			if gap > 0 && gap < fastest {
				fastest = gap
			}
		}
		if fastest != math.MaxFloat64 {
			aq.FastestDoubleSec = round2(fastest)
		}
	}

	// 首杀对枪胜负：FirstKill.Killer 是 target 视为胜，target 是 victim 视为败
	for _, r := range rounds {
		if r.FirstKill == nil {
			continue
		}
		fk := r.FirstKill
		if strings.HasPrefix(fk.Victim, target.Name+"→") {
			aq.OpeningDuelWins++
		} else if strings.HasSuffix(fk.Victim, "→"+target.Name) {
			aq.OpeningDuelLoss++
		}
	}
	return aq
}

// computePressureSplit 区分高压回合（残局/经济局/赛点/连败）vs 普通回合。
func computePressureSplit(target domain.PlayerStats, rounds []domain.RoundSummary, targetID uint64, totalRounds int) *domain.PressureSplit {
	if totalRounds == 0 {
		return nil
	}
	ps := &domain.PressureSplit{}
	losingStreak, curStreak := 0, 0
	highKills, highDeaths := 0, 0
	highADRSum, normADRSum := 0.0, 0.0
	highKAST, normKAST := 0, 0

	myTeam := target.Team
	for i, r := range rounds {
		mp := isMatchPoint(r, rounds, i)
		isHigh := r.ClutchSituation != "" || r.TargetTeamEcon == "eco" || r.TargetTeamEcon == "force" || mp
		// 连败计数：以 target team 视角
		lostByMe := myTeam != "" && r.WinnerTeam != "" && r.WinnerTeam != myTeam
		if lostByMe {
			curStreak++
			if curStreak > losingStreak {
				losingStreak = curStreak
			}
		} else {
			curStreak = 0
		}
		if curStreak >= 3 {
			isHigh = true
		}

		// 单回合 ADR 估算：用 TargetEvents 数量 * 100 + 不准但稳定，本来就只为对比
		killsR := r.TargetKills
		diedR := 0
		if r.TargetDied {
			diedR = 1
		}
		// 用 target.Highlights 做更稳的 ADR 还原成本太高，这里只做相对画像
		if isHigh {
			ps.HighStakeRounds++
			ps.HighStakeKills += killsR
			ps.HighStakeDeaths += diedR
			if r.ClutchSituation != "" {
				ps.ClutchPlayed++
				if strings.Contains(r.ClutchOutcome, "你赢下") {
					ps.ClutchWon++
				}
			}
			if r.TargetTeamEcon == "eco" && killsR > 0 {
				ps.EcoRoundKills += killsR
			}
			if mp {
				ps.MatchPointRounds++
			}
			highADRSum += float64(killsR * 80) // 单回合击杀贡献近似伤害
			if killsR > 0 || !r.TargetDied {
				highKAST++
			}
		} else {
			ps.NormalRounds++
			ps.NormalKills += killsR
			ps.NormalDeaths += diedR
			normADRSum += float64(killsR * 80)
			if killsR > 0 || !r.TargetDied {
				normKAST++
			}
		}
	}
	ps.LosingStreakMax = losingStreak
	if ps.HighStakeRounds > 0 {
		ps.HighStakeADR = round2(highADRSum / float64(ps.HighStakeRounds))
		ps.HighStakeKAST = round2(float64(highKAST) / float64(ps.HighStakeRounds) * 100)
	}
	if ps.NormalRounds > 0 {
		ps.NormalADR = round2(normADRSum / float64(ps.NormalRounds))
		ps.NormalKAST = round2(float64(normKAST) / float64(ps.NormalRounds) * 100)
	}
	_ = highKills
	_ = highDeaths
	_ = targetID
	return ps
}

func isMatchPoint(r domain.RoundSummary, rounds []domain.RoundSummary, idx int) bool {
	// 简化：12 轮制半场决胜（11/12 局）或 13 轮制最后两局。
	rn := r.Number
	switch {
	case rn == 12 || rn == 24:
		return true
	case idx == len(rounds)-1:
		return true
	}
	return false
}

// computeTeamSync 根据所有 player 的 highlights / death 顺序拼出团队配合画像。
func computeTeamSync(stats domain.MatchStats, s *parseState) *domain.TeamSync {
	// 把 target 视角的 trade kill / followup / solo death 聚合
	ts := &domain.TeamSync{}
	tradeGapSum := 0.0
	tradeGapN := 0
	partnerCount := map[string]int{}
	for _, k := range stats.Target.Highlights {
		if k.IsTrade {
			ts.TradeKills++
			tradeGapSum += k.TradeGapSec
			tradeGapN++
			// trade kill 的"谁先倒"通过 victim 的对面阵亡推算，这里粗略用 victim name + zone
			if k.VictimZone != "" {
				partnerCount[k.VictimZone]++
			}
		}
	}
	// trade death：target 自己阵亡后 5s 内有队友补杀
	for _, r := range stats.Rounds {
		if !r.TargetDied {
			continue
		}
		// 队友补杀检查：本回合 target 阵亡时间附近，是否有队友 highlight 在 5s 内
		deathTime := r.TargetDeathTimeSec
		var traded bool
		for _, mate := range stats.Teammates {
			for _, mk := range mate.Highlights {
				if mk.Round != r.Number {
					continue
				}
				gap := mk.TimeSec - deathTime
				if gap > 0 && gap <= 5.0 {
					traded = true
					break
				}
			}
			if traded {
				break
			}
		}
		if traded {
			ts.TradeDeaths++
		} else {
			ts.SoloDeaths++
		}
	}
	if tradeGapN > 0 {
		ts.AvgTradeGapSec = round2(tradeGapSum / float64(tradeGapN))
	}
	totalDeaths := stats.Target.Deaths
	if totalDeaths > 0 {
		ts.TradeRate = round2(float64(ts.TradeDeaths) / float64(totalDeaths) * 100)
	}
	// 跟进击杀：target 击杀后队友 5s 内击杀
	for _, k := range stats.Target.Highlights {
		for _, mate := range stats.Teammates {
			for _, mk := range mate.Highlights {
				if mk.Round != k.Round {
					continue
				}
				gap := mk.TimeSec - k.TimeSec
				if gap > 0 && gap <= 5.0 {
					ts.FollowupKills++
				}
			}
		}
	}
	// 交叉火力：粗略用 zone 距离 + 时间窗口判断
	ts.CrossfireKills = ts.FollowupKills // 简化等价：跟进 = 角度互补
	// 找贡献最多的搭档
	var bestPartner string
	var bestCount int
	for _, mate := range stats.Teammates {
		c := 0
		for _, mk := range mate.Highlights {
			for _, k := range stats.Target.Highlights {
				if mk.Round != k.Round {
					continue
				}
				if math.Abs(mk.TimeSec-k.TimeSec) <= 5.0 {
					c++
				}
			}
		}
		if c > bestCount {
			bestCount = c
			bestPartner = mate.Name
		}
	}
	ts.BestTradePartner = bestPartner
	ts.BestPartnerCount = bestCount
	// 堆点回合：单回合内 3+ 队友死在同一 zone（≥3 视作死堆）
	for _, r := range stats.Rounds {
		if len(r.TeamPushZones) > 0 {
			zone := r.TeamPushZones[0]
			if i := strings.Index(zone, "×"); i > 0 {
				if cnt := atoi(zone[i+len("×"):]); cnt >= 3 {
					ts.StackedDeathRounds++
				}
			}
		}
	}
	return ts
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// computeSmokeReport 比对 target 投出的烟雾落点和地图 keyChoke 标杆。
func computeSmokeReport(mapName string, target domain.PlayerStats, smokes []smokeLanding) *domain.SmokeReport {
	if len(smokes) == 0 {
		return nil
	}
	rep := &domain.SmokeReport{TotalSmokes: len(smokes)}
	keyChokes := smokeKeyChokes(mapName)
	for _, sm := range smokes {
		verdict := "一般"
		detail := sm.zone
		hitKey := false
		for _, kc := range keyChokes {
			if zoneContainsAny(sm.zone, kc.aliases) {
				hitKey = true
				detail = kc.label + "(职业封烟点)"
				break
			}
		}
		if hitKey {
			verdict = "到位"
			rep.AccurateSmokes++
			rep.KeyChokeCovered++
		} else if sm.zone == "" {
			verdict = "落点未识别"
			rep.WastedSmokes++
		} else {
			rep.KeyChokeMissed++
		}
		rep.PerSmokeNotes = append(rep.PerSmokeNotes, domain.SmokeNote{
			Round:       sm.round,
			TimeSec:     round2(sm.timeSec),
			LandingZone: sm.zone,
			Verdict:     verdict,
			Detail:      detail,
		})
	}
	if rep.TotalSmokes > 0 {
		rep.AccuracyPct = round2(float64(rep.AccurateSmokes) / float64(rep.TotalSmokes) * 100)
	}
	return rep
}

type chokeRef struct {
	label   string
	aliases []string
}

// smokeKeyChokes 列出每张地图职业体系下的关键封烟点（zone 子串匹配）。
// 颗粒度按现有 zones.go 给的 zone 名走。
func smokeKeyChokes(mapName string) []chokeRef {
	m := strings.ToLower(strings.TrimPrefix(strings.ToLower(mapName), "de_"))
	switch m {
	case "dust2":
		return []chokeRef{
			{"A 长封烟", []string{"A长"}},
			{"A 大坑控制", []string{"A大坑", "A点"}},
			{"中门封烟", []string{"中门"}},
			{"B 隧道封烟", []string{"B隧道", "B包点", "B门"}},
		}
	case "mirage":
		return []chokeRef{
			{"A 短封烟", []string{"A短"}},
			{"A 点封 CT", []string{"A点"}},
			{"中路 CT 封烟", []string{"中路", "匪徒道"}},
			{"B 公寓封烟", []string{"B点", "B公寓"}},
		}
	case "inferno":
		return []chokeRef{
			{"香蕉道封烟", []string{"香蕉道"}},
			{"A 短封烟", []string{"A短"}},
			{"A 木封烟", []string{"A木", "A点"}},
			{"B 阳台封烟", []string{"B阳台", "B点"}},
		}
	case "ancient":
		return []chokeRef{
			{"A 大房封烟", []string{"A大房", "A点"}},
			{"中路洞穴封烟", []string{"洞穴", "中路"}},
			{"B 坡封烟", []string{"B坡", "B点"}},
		}
	case "anubis":
		return []chokeRef{
			{"A 街封烟", []string{"A街", "A点"}},
			{"中路广场封烟", []string{"广场", "中路"}},
			{"B 水道封烟", []string{"B水道", "B点"}},
		}
	case "nuke":
		return []chokeRef{
			{"外场封烟", []string{"外场"}},
			{"外车库封烟", []string{"外车库"}},
			{"B 下层封烟", []string{"B包点", "下层"}},
		}
	case "overpass":
		return []chokeRef{
			{"A 长封烟", []string{"A长"}},
			{"中路水池封烟", []string{"水池", "中路"}},
			{"B 隧道封烟", []string{"B隧道", "B点"}},
		}
	case "vertigo":
		return []chokeRef{
			{"A 坡道封烟", []string{"A坡道", "A点"}},
			{"中路天梯封烟", []string{"天梯", "中路"}},
			{"B 入口封烟", []string{"B坡道", "B点"}},
		}
	}
	return nil
}

func zoneContainsAny(zone string, aliases []string) bool {
	if zone == "" {
		return false
	}
	for _, a := range aliases {
		if strings.Contains(zone, a) {
			return true
		}
	}
	return false
}

// computeMovementDiscipline 走位纪律：跑动开枪率、孤立死亡、过度突进。
func computeMovementDiscipline(target domain.PlayerStats, rounds []domain.RoundSummary) *domain.MovementDiscipline {
	if len(target.Highlights) == 0 && target.Deaths == 0 {
		return nil
	}
	md := &domain.MovementDiscipline{TotalKills: target.Kills}
	speedSum := 0.0
	for _, k := range target.Highlights {
		if k.WhileMoving {
			md.MovingKills++
		} else {
			md.StationaryKills++
		}
		if k.NoScope {
			md.NoScopeKills++
		}
		// 跳投杀：速度 > 100 视作跳跃中
		if k.KillerSpeed > 100 {
			md.JumpShotKills++
		}
		speedSum += k.KillerSpeed
	}
	if md.TotalKills > 0 {
		md.MovingKillRate = round2(float64(md.MovingKills) / float64(md.TotalKills) * 100)
		md.AvgKillerSpeed = round2(speedSum / float64(md.TotalKills))
	}
	for _, r := range rounds {
		if !r.TargetDied {
			continue
		}
		// overcommit：早期 (<20s) 在远离全队主攻方向的 zone 阵亡
		dt := r.TargetDeathTimeSec - r.StartTimeSec
		if dt > 0 && dt < 20 {
			if isAwayFromTeam(r.TargetDeathZone, r.TeamPushZones) {
				md.OvercommitDeaths++
			}
		}
		// isolation：team_first_death 是 target 自己且队友死在另一片区
		if r.TeamFirstDeath != "" && strings.Contains(r.TeamFirstDeath, target.Name) {
			md.IsolationDeaths++
		}
	}
	return md
}

func isAwayFromTeam(deathZone string, teamPush []string) bool {
	if deathZone == "" || len(teamPush) == 0 {
		return false
	}
	for _, z := range teamPush {
		if i := strings.Index(z, "×"); i > 0 {
			z = z[:i]
		}
		if z == deathZone || strings.Contains(deathZone, z) || strings.Contains(z, deathZone) {
			return false
		}
	}
	return true
}

// 用于把 atoi 等小工具集中。
var _ = fmt.Sprint
