package game

import "testing"

func TestGrowPlanetsLockedUsesSlowerInterval(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		tickRate: DefaultIdleTickRate,
		planets: map[int]*Planet{
			1: {ID: 1, Owner: 1, Ships: 10, Growth: 3},
		},
	}

	for range DefaultIdleTickRate {
		engine.growPlanetsLocked()
	}
	if ships := engine.planets[1].Ships; ships != 10 {
		t.Fatalf("expected no growth after 1 second, got %d ships", ships)
	}

	engine.tickRate = DefaultTickRate
	for range DefaultTickRate {
		engine.growPlanetsLocked()
	}
	if ships := engine.planets[1].Ships; ships != 13 {
		t.Fatalf("expected growth after slower interval, got %d ships", ships)
	}
}
