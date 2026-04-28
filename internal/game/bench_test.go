package game

import (
	"slices"
	"testing"
)

// BenchmarkAdvanceHighLoad measures the cost of one Advance() call when the
// world is saturated: 12 players, maximum planet count, and every player
// sending 100% of ships to every other player's home planet so the fleet
// population is as large as possible.
func BenchmarkAdvanceHighLoad(b *testing.B) {
	engine := NewEngineWithConfig(MapConfig{PlayerCount: 12})

	// Collect home planets by owner so each player can attack all others.
	homePlanets := make(map[int]int) // ownerID -> planetID
	for id, planet := range engine.planets {
		if planet.Owner != 0 && planet.Growth == homePlanetGrowth {
			homePlanets[planet.Owner] = id
		}
	}

	// Give every owned planet a large ship count so fleets keep launching.
	for _, planet := range engine.planets {
		if planet.Owner != 0 {
			planet.Ships = 500
		}
	}

	sendFleets := func() {
		for attackerID, sourceID := range homePlanets {
			for defenderID, targetID := range homePlanets {
				if attackerID == defenderID {
					continue
				}
				engine.planets[sourceID].Ships += 50
				//nolint:errcheck
				engine.SendFleet(attackerID, sourceID, targetID, 50)
			}
		}
	}

	// Warm up: fill the world with fleets before we start timing.
	for range 30 {
		sendFleets()
		engine.Advance(1 / float64(DefaultTickRate)) //nolint:errcheck
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Re-seed fleets every 20 ticks so the fleet population stays high
		// throughout the benchmark rather than draining to zero.
		if b.N > 1 {
			sendFleets()
		}
		engine.Advance(1 / float64(DefaultTickRate)) //nolint:errcheck
	}
}

// BenchmarkAdvanceFirstTick measures the cost of the very first Advance() call
// after a mass launch from many owned planets. Setup: 30 planets owned by
// player 1 with 200 ships each, plus one neutral target planet. Player 1 sends
// 50% of each planet's ships to the target — 30 × 100 = 3000 ships spawned in
// one shot. The engine is rebuilt outside the timer each iteration so every run
// sees the exact same cold-start conditions.
func BenchmarkAdvanceFirstTick(b *testing.B) {
	const (
		sourcePlanetCount = 30
		shipsPerPlanet    = 200
		launchPercentage  = 50
		planetRadius      = 20.0
		planetSpacing     = 120.0
		cols              = 6
	)

	buildEngine := func() (*Engine, int) {
		planets := make(map[int]*Planet, sourcePlanetCount+1)

		// Target: neutral planet in the centre of the grid.
		rows := (sourcePlanetCount + cols - 1) / cols
		gridW := float64(cols) * planetSpacing
		gridH := float64(rows) * planetSpacing
		targetID := sourcePlanetCount + 1
		planets[targetID] = &Planet{
			ID:     targetID,
			X:      gridW / 2,
			Y:      gridH / 2,
			Radius: planetRadius,
			Owner:  0,
			Ships:  0,
			Growth: 1,
		}

		// Source planets laid out in a grid, owned by player 1.
		for i := range sourcePlanetCount {
			id := i + 1
			col := i % cols
			row := i / cols
			x := float64(col)*planetSpacing + planetSpacing/2
			y := float64(row)*planetSpacing + planetSpacing/2
			planets[id] = &Planet{
				ID:     id,
				X:      x,
				Y:      y,
				Radius: planetRadius,
				Owner:  1,
				Ships:  shipsPerPlanet,
				Growth: 2,
			}
		}

		sortedIDs := make([]int, 0, len(planets))
		for id := range planets {
			sortedIDs = append(sortedIDs, id)
		}
		slices.Sort(sortedIDs)

		engine := &Engine{
			tickRate:        DefaultTickRate,
			fleetSpeed:      defaultFleetSpeedUPS,
			worldWidth:      gridW + planetSpacing,
			worldHeight:     gridH + planetSpacing,
			planets:         planets,
			planetIndex:     newPlanetSpatialIndex(planets),
			sortedPlanetIDs: sortedIDs,
			fleets:          make(map[int]*Fleet),
			sortedFleetIDs:  make([]int, 0),
			nextFleetID:     1,
			mapName:         "bench-first-tick",
		}
		return engine, targetID
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		b.StopTimer()
		engine, targetID := buildEngine()
		for i := range sourcePlanetCount {
			sourceID := i + 1
			//nolint:errcheck
			engine.SendFleet(1, sourceID, targetID, launchPercentage)
		}
		b.StartTimer()

		engine.Advance(1 / float64(DefaultTickRate)) //nolint:errcheck
	}
}

// BenchmarkSnapshotHighLoad measures the per-player snapshot cost under the
// same high-load conditions as BenchmarkAdvanceHighLoad.
func BenchmarkSnapshotHighLoad(b *testing.B) {
	engine := NewEngineWithConfig(MapConfig{PlayerCount: 12})

	homePlanets := make(map[int]int)
	for id, planet := range engine.planets {
		if planet.Owner != 0 && planet.Growth == homePlanetGrowth {
			homePlanets[planet.Owner] = id
		}
	}

	for _, planet := range engine.planets {
		if planet.Owner != 0 {
			planet.Ships = 500
		}
	}

	for range 60 {
		for attackerID, sourceID := range homePlanets {
			for defenderID, targetID := range homePlanets {
				if attackerID == defenderID {
					continue
				}
				engine.planets[sourceID].Ships += 50
				//nolint:errcheck
				engine.SendFleet(attackerID, sourceID, targetID, 50)
			}
		}
		engine.Advance(1 / float64(DefaultTickRate)) //nolint:errcheck
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		// Cycle through all 12 players to represent the real per-tick broadcast cost.
		playerID := (i % 12) + 1
		engine.SnapshotForPlayer(playerID)
	}
}
