package analyzer

import "testing"

func TestCleanTerms(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"pistol局", "pistol局，你方装备4300", "手枪局，你方装备4300"},
		{"force-buy", "这把是 force-buy", "这把是 强起局"},
		{"裸 force", "force 起，配 USP-S", "强起局 起，配 USP-S"},
		{"裸 eco", "eco 回合不要乱冲", "经济局 回合不要乱冲"},
		{"semi buy", "对手 semi buy", "对手 半起局"},
		{"full buy", "你方 full-buy", "你方 全装局"},
		{"Clutch", "Clutch 1v3 完美", "残局 1v3 完美"},
		{"keep weapon names", "USP-S 连续 HS", "USP-S 连续 HS"},
		{"forceful 不替换", "执行 forceful 一些", "执行 forceful 一些"},
		{"force局 整体替换", "这是 force局 节奏", "这是 强起局 节奏"},
		{"句首 pistol", "pistol 核心是控制移动", "手枪局 核心是控制移动"},
		{"末尾 eco", "回合是 eco", "回合是 经济局"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cleanTerms(c.in)
			if got != c.want {
				t.Errorf("cleanTerms(%q)\n  got=  %q\n  want= %q", c.in, got, c.want)
			}
		})
	}
}
