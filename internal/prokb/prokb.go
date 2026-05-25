package prokb

import (
	"strings"

	"github.com/cs2demo/platform/internal/domain"
)

type KB interface {
	Lookup(mapName, role string) domain.ProBaseline
	RoleHints(stats domain.PlayerStats) string
}

type Static struct{}

func New() *Static { return &Static{} }

func (s *Static) Lookup(mapName, role string) domain.ProBaseline {
	mapName = normalizeMap(mapName)
	role = strings.ToLower(role)

	base, ok := proBaselines[role]
	if !ok {
		base = proBaselines["rifler"]
	}
	if mb, ok := mapAdjustments[mapName]; ok {
		base.ADR += mb.ADRDelta
		base.UtilityDamage += mb.UtilDelta
	}
	base.Map = mapName
	base.Role = role
	return base
}

func (s *Static) RoleHints(p domain.PlayerStats) string {
	awp := p.WeaponKills["AWP"] + p.WeaponKills["awp"]
	totalKills := p.Kills
	if totalKills > 0 && float64(awp)/float64(totalKills) > 0.45 {
		return "awper"
	}
	if p.EntryKills >= 3 || (p.EntryDeaths >= 3 && p.Team == "T") {
		return "entry"
	}
	if p.UtilityDamage >= 80 {
		return "support"
	}
	return "rifler"
}

func normalizeMap(m string) string {
	m = strings.ToLower(m)
	switch m {
	case "de_mirage", "mirage":
		return "mirage"
	case "de_inferno", "inferno":
		return "inferno"
	case "de_dust2", "dust2":
		return "dust2"
	case "de_nuke", "nuke":
		return "nuke"
	case "de_overpass", "overpass":
		return "overpass"
	case "de_ancient", "ancient":
		return "ancient"
	case "de_anubis", "anubis":
		return "anubis"
	case "de_vertigo", "vertigo":
		return "vertigo"
	}
	return m
}

var proBaselines = map[string]domain.ProBaseline{
	"rifler": {
		Role:          "rifler",
		ADR:           82,
		KAST:          73,
		HeadshotPct:   48,
		UtilityDamage: 65,
		EntryKillRate: 0.18,
		Notes:         "Tier-1 步枪手参考：稳定 ADR 80+，KAST 70+，回合参与度优先于个人击杀",
	},
	"awper": {
		Role:          "awper",
		ADR:           88,
		KAST:          70,
		HeadshotPct:   35,
		UtilityDamage: 25,
		EntryKillRate: 0.22,
		Notes:         "顶级 AWP 选手参考：开局位卡视野，残局保枪率高，掉枪节奏不慌",
	},
	"entry": {
		Role:          "entry",
		ADR:           78,
		KAST:          68,
		HeadshotPct:   52,
		UtilityDamage: 50,
		EntryKillRate: 0.45,
		Notes:         "突破手参考：抢首杀概率高于 40%，死亡率较高但能为队伍创造空间",
	},
	"support": {
		Role:          "support",
		ADR:           70,
		KAST:          78,
		HeadshotPct:   45,
		UtilityDamage: 110,
		EntryKillRate: 0.10,
		Notes:         "辅助/投手参考：投掷物伤害 100+，闪光助攻多，KAST 比击杀更重要",
	},
	"igl": {
		Role:          "igl",
		ADR:           65,
		KAST:          72,
		HeadshotPct:   42,
		UtilityDamage: 80,
		EntryKillRate: 0.08,
		Notes:         "指挥参考：节奏控制 > 个人数据，残局判断与经济管理是核心",
	},
}

type mapDelta struct {
	ADRDelta  float64
	UtilDelta int
}

var mapAdjustments = map[string]mapDelta{
	"mirage":   {ADRDelta: 2, UtilDelta: 5},
	"inferno":  {ADRDelta: -2, UtilDelta: 15},
	"dust2":    {ADRDelta: 4, UtilDelta: -10},
	"nuke":     {ADRDelta: -3, UtilDelta: 0},
	"overpass": {ADRDelta: 0, UtilDelta: 10},
	"ancient":  {ADRDelta: -1, UtilDelta: 5},
	"anubis":   {ADRDelta: 1, UtilDelta: 5},
	"vertigo":  {ADRDelta: -2, UtilDelta: -5},
}
