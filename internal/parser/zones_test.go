package parser

import "testing"

func TestZoneAtFallsBackToHeader(t *testing.T) {
	s := &parseState{}
	if mapZone("", 100, 100) != "" {
		t.Fatal("mapZone with empty mapName should return empty")
	}
	s.mapName = "de_ancient"
	if got := mapZone(s.mapName, 200, 1500); got == "" {
		t.Errorf("ancient zone lookup returned empty for (200,1500)")
	}
}

func TestZoneAncientCoverage(t *testing.T) {
	cases := []struct {
		name string
		x, y float64
	}{
		{"A大房 quadrant", 200, 1500},
		{"A坡道", -500, 1200},
		{"中路", 0, 0},
		{"B坡", 200, -800},
		{"B底", -500, -800},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			z := zoneAncient(c.x, c.y)
			if z == "" {
				t.Errorf("ancient (%v,%v) returned empty zone", c.x, c.y)
			}
		})
	}
}
