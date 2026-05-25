package parser

import "strings"

func mapZone(mapName string, x, y float64) string {
	mapName = strings.ToLower(mapName)
	mapName = strings.TrimPrefix(mapName, "de_")
	switch mapName {
	case "dust2":
		return zoneDust2(x, y)
	case "mirage":
		return zoneMirage(x, y)
	case "inferno":
		return zoneInferno(x, y)
	case "nuke":
		return zoneNuke(x, y)
	case "ancient":
		return zoneAncient(x, y)
	case "anubis":
		return zoneAnubis(x, y)
	case "overpass":
		return zoneOverpass(x, y)
	case "vertigo":
		return zoneVertigo(x, y)
	}
	return ""
}

func zoneDust2(x, y float64) string {
	if y > 1500 {
		if x > 800 {
			return "A长"
		}
		if x > -300 {
			return "A大坑/A点"
		}
		return "A小/CT楼梯"
	}
	if y > 0 {
		if x > 800 {
			return "中门T方"
		}
		if x > -1000 {
			return "中路"
		}
		return "B隧道入口"
	}
	if y > -1500 {
		if x > 800 {
			return "T出生点"
		}
		if x < -1500 {
			return "B隧道"
		}
		return "中路低区"
	}
	if x < -1500 {
		return "B包点/B门"
	}
	return "B平台"
}

func zoneMirage(x, y float64) string {
	if x > 0 {
		if y > 0 {
			return "A点"
		}
		if y > -1500 {
			return "A短/中路"
		}
		return "T出生/A长入口"
	}
	if y > 500 {
		return "B点/B公寓"
	}
	if y > -800 {
		return "中路/匪徒道"
	}
	return "B底"
}

func zoneInferno(x, y float64) string {
	if x > 1500 {
		if y > 0 {
			return "A点/A木"
		}
		return "A短"
	}
	if x > 0 {
		if y > 500 {
			return "香蕉道"
		}
		return "中路/T出生"
	}
	if y > 0 {
		return "B点/B阳台"
	}
	return "B底/CT出生"
}

func zoneNuke(x, y float64) string {
	if y > 0 {
		if x > 0 {
			return "外场/外平台"
		}
		return "外车库/T出生"
	}
	if x > 0 {
		return "A平台/上层"
	}
	return "B包点/下层"
}

func zoneAncient(x, y float64) string {
	if y > 1000 {
		if x > 0 {
			return "A点/A大房"
		}
		return "A坡道/连接处"
	}
	if y > -500 {
		return "中路/洞穴"
	}
	if x > 0 {
		return "B点/B坡"
	}
	return "B底/T出生"
}

func zoneAnubis(x, y float64) string {
	if y > 500 {
		if x > 0 {
			return "A点/A街"
		}
		return "A连接"
	}
	if y > -800 {
		return "中路/广场"
	}
	return "B点/B水道"
}

func zoneOverpass(x, y float64) string {
	if y > 0 {
		if x > 0 {
			return "A长/A连接"
		}
		return "A点"
	}
	if x > 0 {
		return "中路/水池"
	}
	return "B点/B隧道"
}

func zoneVertigo(x, y float64) string {
	if y > 1500 {
		return "A点/A坡道"
	}
	if y > 0 {
		return "中路/天梯"
	}
	return "B点/B坡道"
}
