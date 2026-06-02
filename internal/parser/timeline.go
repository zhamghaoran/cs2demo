package parser

import (
	"fmt"
	"sort"

	"github.com/cs2demo/platform/internal/domain"
)

// roundTimelineState 收集单回合的关键事件 + 阶段信号，最终在 RoundEnd 收尾时归档。
type roundTimelineState struct {
	startSec        float64
	firstContactSec float64
	bombPlantSec    float64
	bombDefuseSec   float64
	clutchStartSec  float64
	executeStartSec float64
	endSec          float64
	winnerTeam      string
	endReason       string
	bombPlanted     bool
	bombDefused     bool
	events          []domain.TimelineEvent
}

func (s *parseState) ensureTimeline(roundIdx int) *roundTimelineState {
	if s.timelines == nil {
		s.timelines = map[int]*roundTimelineState{}
	}
	t, ok := s.timelines[roundIdx]
	if !ok {
		t = &roundTimelineState{}
		s.timelines[roundIdx] = t
	}
	return t
}

// recordTimelineEvent 用 round-relative 秒数（开局后多少秒）打点，方便 LLM 解读节奏。
func (s *parseState) recordTimelineEvent(roundIdx int, absSec float64, kind, detail, side, zone string) {
	t := s.ensureTimeline(roundIdx)
	rel := absSec - t.startSec
	if rel < 0 {
		rel = 0
	}
	t.events = append(t.events, domain.TimelineEvent{
		TimeSec: round2(rel),
		Kind:    kind,
		Detail:  detail,
		Side:    side,
		Zone:    zone,
	})
}

// finalizeTimelines 在 finalize 阶段合并到 RoundSummary 上。
func (s *parseState) finalizeTimelines(rounds []domain.RoundSummary) {
	for i := range rounds {
		r := &rounds[i]
		t, ok := s.timelines[r.Number]
		if !ok || t == nil {
			continue
		}
		sort.SliceStable(t.events, func(a, b int) bool {
			return t.events[a].TimeSec < t.events[b].TimeSec
		})
		tl := &domain.RoundTimeline{
			KeyEvents: t.events,
		}
		// default 阶段：开局到 first contact 或 first util 之间
		// execute：第一次 grenade / first kill 后 5s 内的强度
		// post-plant：bomb planted 之后
		// clutch：1vN 触发点
		if t.firstContactSec > 0 {
			tl.DefaultEndSec = round2(t.firstContactSec - t.startSec)
		} else if r.FirstContactSec > 0 {
			tl.DefaultEndSec = round2(r.FirstContactSec)
		}
		if t.executeStartSec > 0 {
			tl.ExecuteStartSec = round2(t.executeStartSec - t.startSec)
		}
		if t.bombPlantSec > 0 {
			tl.PostPlantSec = round2(t.bombPlantSec - t.startSec)
		}
		if t.clutchStartSec > 0 {
			tl.ClutchStartSec = round2(t.clutchStartSec - t.startSec)
		}
		tl.Phase = inferRoundPhase(tl, r)
		tl.PaceProfile = inferPaceProfile(tl, r)
		r.Timeline = tl
	}
}

// inferRoundPhase 给 LLM 一个回合性质标签。
func inferRoundPhase(tl *domain.RoundTimeline, r *domain.RoundSummary) string {
	switch {
	case r.ClutchSituation != "":
		return "残局"
	case r.BombPlanted:
		return "下包+post-plant"
	case tl.DefaultEndSec >= 35:
		return "default 拖时间"
	case tl.DefaultEndSec > 0 && tl.DefaultEndSec < 15:
		return "速攻 rush"
	case r.TargetTeamEcon == "eco" || r.OpponentTeamEcon == "eco":
		return "经济局博弈"
	}
	return "标准对枪"
}

// inferPaceProfile 用首接触时间分桶 + execute 节奏判断。
func inferPaceProfile(tl *domain.RoundTimeline, r *domain.RoundSummary) string {
	first := tl.DefaultEndSec
	if first <= 0 && r.FirstContactSec > 0 {
		first = r.FirstContactSec
	}
	switch {
	case first <= 0:
		return "无交火"
	case first < 12:
		return fmt.Sprintf("rush 节奏(首接触%.0fs)", first)
	case first < 25:
		return fmt.Sprintf("正常 execute(首接触%.0fs)", first)
	case first < 50:
		return fmt.Sprintf("default 控图(首接触%.0fs)", first)
	}
	return fmt.Sprintf("拖时间残局(首接触%.0fs)", first)
}
