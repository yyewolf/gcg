package game

import "testing"

func TestResolveDynamicTickRateDefaultsToIdle(t *testing.T) {
	t.Parallel()

	engine := &Engine{tickRate: DefaultIdleTickRate, planets: map[int]*Planet{}}

	if tickRate := engine.resolveDynamicTickRate(); tickRate != DefaultIdleTickRate {
		t.Fatalf("expected idle tickrate %d, got %d", DefaultIdleTickRate, tickRate)
	}
}

func TestResolveDynamicTickRateRaisesNearArrival(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		tickRate:   DefaultIdleTickRate,
		fleetSpeed: defaultFleetSpeedUPS,
		planets: map[int]*Planet{
			1: {ID: 1, X: 0, Y: 0, Radius: 15},
			2: {ID: 2, X: 32, Y: 0, Radius: 15},
		},
		fleets: map[int]*Fleet{
			1: {ID: 1, TargetID: 2, X: 0, Y: 0, VX: defaultFleetSpeedUPS, VY: 0},
		},
	}
	engine.planetIndex = newPlanetSpatialIndex(engine.planets)

	if tickRate := engine.resolveDynamicTickRate(); tickRate != DefaultTickRate {
		t.Fatalf("expected high tickrate %d near arrival, got %d", DefaultTickRate, tickRate)
	}
}
