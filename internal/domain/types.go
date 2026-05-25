package domain

import "time"

type DemoStatus string

const (
	StatusQueued    DemoStatus = "queued"
	StatusParsing   DemoStatus = "parsing"
	StatusAnalyzing DemoStatus = "analyzing"
	StatusDone      DemoStatus = "done"
	StatusFailed    DemoStatus = "failed"
)

type Demo struct {
	ID         string     `json:"id"`
	Filename   string     `json:"filename"`
	FilePath   string     `json:"-"`
	TargetUser string     `json:"target_user"`
	Status     DemoStatus `json:"status"`
	Error      string     `json:"error,omitempty"`
	Candidates []string   `json:"candidates,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type MatchStats struct {
	Map         string         `json:"map"`
	DurationSec int            `json:"duration_sec"`
	TickRate    float64        `json:"tick_rate"`
	RoundsTotal int            `json:"rounds_total"`
	ScoreT      int            `json:"score_t"`
	ScoreCT     int            `json:"score_ct"`
	Target      PlayerStats    `json:"target"`
	Teammates   []PlayerStats  `json:"teammates,omitempty"`
	Opponents   []PlayerStats  `json:"opponents,omitempty"`
	AllPlayers  []string       `json:"all_players,omitempty"`
	Rounds      []RoundSummary `json:"rounds,omitempty"`
}

type PlayerStats struct {
	Name           string             `json:"name"`
	SteamID        uint64             `json:"steam_id"`
	Team           string             `json:"team"`
	Kills          int                `json:"kills"`
	Deaths         int                `json:"deaths"`
	Assists        int                `json:"assists"`
	HeadshotKills  int                `json:"headshot_kills"`
	HeadshotPct    float64            `json:"headshot_pct"`
	ADR            float64            `json:"adr"`
	KAST           float64            `json:"kast"`
	UtilityDamage  int                `json:"utility_damage"`
	FlashAssists   int                `json:"flash_assists"`
	EntryKills     int                `json:"entry_kills"`
	EntryDeaths    int                `json:"entry_deaths"`
	ClutchAttempts int                `json:"clutch_attempts"`
	ClutchWins     int                `json:"clutch_wins"`
	WeaponKills    map[string]int     `json:"weapon_kills,omitempty"`
	GrenadeUsage   map[string]int     `json:"grenade_usage,omitempty"`
	Highlights     []KillEvent        `json:"highlights,omitempty"`
	Mistakes       []NotableEvent     `json:"mistakes,omitempty"`
	Economy        EconomySummary     `json:"economy"`
}

type EconomySummary struct {
	AvgMoneySpentPerRound int `json:"avg_money_spent_per_round"`
	EcoRoundsKills        int `json:"eco_round_kills"`
	ForceBuyRoundsKills   int `json:"force_buy_round_kills"`
}

type RoundSummary struct {
	Number       int          `json:"number"`
	StartTimeSec float64      `json:"start_time_sec"`
	EndTimeSec   float64      `json:"end_time_sec"`
	WinnerTeam   string       `json:"winner_team"`
	EndReason    string       `json:"end_reason"`
	TargetKills  int          `json:"target_kills"`
	TargetDied   bool         `json:"target_died"`
	TargetDeathTimeSec float64 `json:"target_death_time_sec,omitempty"`
	FirstKill    *KillEvent   `json:"first_kill,omitempty"`
	TargetEvents []KillEvent  `json:"target_events,omitempty"`
	BombPlanted  bool         `json:"bomb_planted"`
	BombDefused  bool         `json:"bomb_defused"`

	TargetTeamEcon   string `json:"target_team_econ"`
	OpponentTeamEcon string `json:"opponent_team_econ"`
	TargetTeamEquipValue   int `json:"target_team_equip_value"`
	OpponentTeamEquipValue int `json:"opponent_team_equip_value"`
	FirstContactSec        float64 `json:"first_contact_sec,omitempty"`
	TargetStartMoney       int `json:"target_start_money"`

	ClutchSituation string `json:"clutch_situation,omitempty"`
	ClutchOutcome   string `json:"clutch_outcome,omitempty"`

	TargetDeathZone string `json:"target_death_zone,omitempty"`
	TeamPushZones   []string `json:"team_push_zones,omitempty"`
	TeamFirstDeath  string `json:"team_first_death,omitempty"`
	TeamLastAlive   string `json:"team_last_alive,omitempty"`
	BombSiteZone    string `json:"bomb_site_zone,omitempty"`

	TargetActions   *TargetRoundActions `json:"target_actions,omitempty"`
	OpponentContext *OpponentContext    `json:"opponent_context,omitempty"`

	Analysis *RoundAnalysis `json:"analysis,omitempty"`
}

type TargetRoundActions struct {
	Grenades       []GrenadeEvent     `json:"grenades,omitempty"`
	PositionTrack  []PositionSample   `json:"position_track,omitempty"`
	FlashAssists   []FlashAssistEvent `json:"flash_assists,omitempty"`
	UtilityDamage  int                `json:"utility_damage"`
	ZoneOccupancy  map[string]float64 `json:"zone_occupancy,omitempty"`
	ControlScore   int                `json:"control_score"`
}

type GrenadeEvent struct {
	Type           string  `json:"type"`
	ThrownAtSec    float64 `json:"thrown_at_sec"`
	ThrowerZone    string  `json:"thrower_zone,omitempty"`
	LandingZone    string  `json:"landing_zone,omitempty"`
	DamageDealt    int     `json:"damage_dealt,omitempty"`
	EnemiesFlashed int     `json:"enemies_flashed,omitempty"`
	TeamFlashed    int     `json:"team_flashed,omitempty"`
	FlashDuration  float64 `json:"flash_duration,omitempty"`
}

type FlashAssistEvent struct {
	Round       int     `json:"round"`
	TimeSec     float64 `json:"time_sec"`
	VictimName  string  `json:"victim_name"`
	VictimZone  string  `json:"victim_zone,omitempty"`
	AssistedKill bool   `json:"assisted_kill"`
}

type PositionSample struct {
	TimeSec float64 `json:"time_sec"`
	Zone    string  `json:"zone"`
}

type OpponentContext struct {
	RecentBombSites    []string `json:"recent_bomb_sites,omitempty"`
	RecentDeathZones   []string `json:"recent_death_zones,omitempty"`
	RecentEcons        []string `json:"recent_econs,omitempty"`
	PredictedIntent    string   `json:"predicted_intent,omitempty"`
	PredictionEvidence string   `json:"prediction_evidence,omitempty"`
}

type RoundAnalysis struct {
	Tactic   string `json:"tactic"`
	Mistake  string `json:"mistake,omitempty"`
	Clutch   string `json:"clutch,omitempty"`
	Verdict  string `json:"verdict"`
}

type KillEvent struct {
	Round       int     `json:"round"`
	TimeSec     float64 `json:"time_sec"`
	Weapon      string  `json:"weapon"`
	Victim      string  `json:"victim"`
	Headshot    bool    `json:"headshot"`
	ThroughSmoke bool   `json:"through_smoke"`
	NoScope     bool    `json:"no_scope"`
	Distance    float64 `json:"distance"`
	KillerZone  string  `json:"killer_zone,omitempty"`
	VictimZone  string  `json:"victim_zone,omitempty"`
}

type NotableEvent struct {
	Round   int     `json:"round"`
	TimeSec float64 `json:"time_sec"`
	Kind    string  `json:"kind"`
	Detail  string  `json:"detail"`
}

type ProBaseline struct {
	Role          string  `json:"role"`
	Map           string  `json:"map"`
	ADR           float64 `json:"adr"`
	KAST          float64 `json:"kast"`
	HeadshotPct   float64 `json:"headshot_pct"`
	UtilityDamage int     `json:"utility_damage"`
	EntryKillRate float64 `json:"entry_kill_rate"`
	Notes         string  `json:"notes,omitempty"`
}

type AnalysisReport struct {
	DemoID       string          `json:"demo_id"`
	GeneratedAt  time.Time       `json:"generated_at"`
	OverallScore int             `json:"overall_score"`
	Verdict      string          `json:"verdict"`
	Strengths    []ReportPoint   `json:"strengths"`
	Weaknesses   []ReportPoint   `json:"weaknesses"`
	Suggestions  []ReportPoint   `json:"suggestions"`
	Comparison   []MetricCompare `json:"comparison"`
	RoundAnalyses []RoundAnalysisOut `json:"round_analyses,omitempty"`
	ProReference string          `json:"pro_reference,omitempty"`
}

type RoundAnalysisOut struct {
	Round           int    `json:"round"`
	Tactic          string `json:"tactic"`
	Mistake         string `json:"mistake,omitempty"`
	Clutch          string `json:"clutch,omitempty"`
	GrenadeEval     string `json:"grenade_eval,omitempty"`
	MapControl      string `json:"map_control,omitempty"`
	UtilityAssist   string `json:"utility_assist,omitempty"`
	OpponentPredict string `json:"opponent_predict,omitempty"`
	Adjustment      string `json:"adjustment,omitempty"`
	Verdict         string `json:"verdict"`
}

type ReportPoint struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Round  int    `json:"round,omitempty"`
}

type MetricCompare struct {
	Metric    string  `json:"metric"`
	You       float64 `json:"you"`
	ProMedian float64 `json:"pro_median"`
	Verdict   string  `json:"verdict"`
}
