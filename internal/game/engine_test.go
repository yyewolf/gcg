package game

import (
	"math"
	"testing"
)

func TestSendFleetSpawnsShipsAroundSourcePlanet(t *testing.T) {
	t.Parallel()
	engine := &Engine{
		tickRate:   DefaultTickRate,
		fleetSpeed: defaultFleetSpeedUPS,
		planets: map[int]*Planet{
			1: {ID: 1, X: 0, Y: 0, Radius: 15, Owner: 1, Ships: 40},
			2: {ID: 2, X: 220, Y: 0, Radius: 15, Owner: 0, Ships: 10},
		},
		fleets:      make(map[int]*Fleet),
		nextFleetID: 1,
		mapName:     "test",
	}

	_, err := engine.SendFleet(1, 1, 2, 100)
	if err != nil {
		t.Fatalf("send fleet: %v", err)
	}

	if len(engine.fleets) != 40 {
		t.Fatalf("expected 40 backend fleets, got %d", len(engine.fleets))
	}

	launchRadius := engine.planets[1].Radius + fleetCollisionRadius + collisionPadding
	foundLeftOfSource := false
	foundRightOfSource := false

	for _, fleet := range engine.fleets {
		distance := math.Hypot(fleet.X-engine.planets[1].X, fleet.Y-engine.planets[1].Y)
		if distance < launchRadius-launchPositionTolerance {
			t.Fatalf("fleet spawned inside launch radius: distance=%.3f launchRadius=%.3f", distance, launchRadius)
		}

		if fleet.X < engine.planets[1].X {
			foundLeftOfSource = true
		}
		if fleet.X > engine.planets[1].X {
			foundRightOfSource = true
		}
	}

	if !foundLeftOfSource || !foundRightOfSource {
		t.Fatal("expected circular launch distribution around the source planet, but fleets stayed on one side")
	}
}

func TestSendFleetBundlesLargeLaunchBeforeFirstTick(t *testing.T) {
	t.Parallel()
	engine := &Engine{
		tickRate:   DefaultTickRate,
		fleetSpeed: defaultFleetSpeedUPS,
		planets: map[int]*Planet{
			1: {ID: 1, X: 0, Y: 0, Radius: 15, Owner: 1, Ships: 2400},
			2: {ID: 2, X: 600, Y: 0, Radius: 15, Owner: 0, Ships: 10},
		},
		fleets:      make(map[int]*Fleet),
		nextFleetID: 1,
		mapName:     "test",
	}

	_, err := engine.SendFleet(1, 1, 2, 100)
	if err != nil {
		t.Fatalf("send fleet: %v", err)
	}

	if len(engine.fleets) >= 2400 {
		t.Fatalf("expected launch-time bundling to reduce spawned fleet count, got %d", len(engine.fleets))
	}

	maxBundleSize := launchFleetBundleSize(0, 2400)
	foundBundledFleet := false
	for _, fleet := range engine.fleets {
		if fleet.Ships > maxBundleSize {
			t.Fatalf("expected bundle size <= %d, got %d", maxBundleSize, fleet.Ships)
		}
		if fleet.Ships > 1 {
			foundBundledFleet = true
		}
	}

	if !foundBundledFleet {
		t.Fatal("expected large launch to create bundled fleets before the first tick")
	}
}

func TestFleetSlidesAroundIntermediatePlanet(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		tickRate:   DefaultTickRate,
		fleetSpeed: defaultFleetSpeedUPS,
		planets: map[int]*Planet{
			1: {ID: 1, X: 0, Y: 0, Radius: 15, Owner: 1, Ships: 2},
			2: {ID: 2, X: 220, Y: 0, Radius: 15, Owner: 0, Ships: 10},
			3: {ID: 3, X: 110, Y: 0, Radius: 22, Owner: 0, Ships: 0},
		},
		fleets:      make(map[int]*Fleet),
		nextFleetID: 1,
		mapName:     "test",
	}

	fleet, err := engine.SendFleet(1, 1, 2, 50)
	if err != nil {
		t.Fatalf("send fleet: %v", err)
	}

	maxAbsY := 0.0
	arrived := false
	minimumClearance := avoidanceRadius(engine.planets[3]) - 0.001

	for range 180 {
		engine.Advance()

		activeFleet, ok := engine.fleets[fleet.ID]
		if !ok {
			arrived = true
			break
		}

		if distanceSquared(activeFleet.X, activeFleet.Y, engine.planets[3].X, engine.planets[3].Y) < minimumClearance*minimumClearance {
			t.Fatalf("fleet penetrated obstacle clearance radius: distanceSquared=%.2f", distanceSquared(activeFleet.X, activeFleet.Y, engine.planets[3].X, engine.planets[3].Y))
		}

		if activeFleet.Y < 0 && -activeFleet.Y > maxAbsY {
			maxAbsY = -activeFleet.Y
		}
		if activeFleet.Y > maxAbsY {
			maxAbsY = activeFleet.Y
		}
	}

	if !arrived {
		t.Fatal("expected fleet to eventually reach the target")
	}

	if maxAbsY < 5 {
		t.Fatalf("expected curved path around the obstacle, maxAbsY=%.2f", maxAbsY)
	}
}

func TestFleetChoosesShortestSideAroundOffsetPlanet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obstacleY   float64
		expectAbove bool
	}{
		{name: "planet above path", obstacleY: 18, expectAbove: false},
		{name: "planet below path", obstacleY: -18, expectAbove: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			engine := &Engine{
				tickRate:   DefaultTickRate,
				fleetSpeed: defaultFleetSpeedUPS,
				planets: map[int]*Planet{
					1: {ID: 1, X: 0, Y: 0, Radius: 15, Owner: 1, Ships: 2},
					2: {ID: 2, X: 220, Y: 0, Radius: 15, Owner: 0, Ships: 10},
					3: {ID: 3, X: 110, Y: testCase.obstacleY, Radius: 22, Owner: 0, Ships: 0},
				},
				fleets:      make(map[int]*Fleet),
				nextFleetID: 1,
				mapName:     "test",
			}

			fleet, err := engine.SendFleet(1, 1, 2, 50)
			if err != nil {
				t.Fatalf("send fleet: %v", err)
			}

			maxExpectedDeviation := 0.0
			maxWrongDeviation := 0.0
			arrived := false

			for range 180 {
				engine.Advance()

				activeFleet, ok := engine.fleets[fleet.ID]
				if !ok {
					arrived = true
					break
				}

				if testCase.expectAbove {
					if activeFleet.Y > maxExpectedDeviation {
						maxExpectedDeviation = activeFleet.Y
					}
					if -activeFleet.Y > maxWrongDeviation {
						maxWrongDeviation = -activeFleet.Y
					}
				} else {
					if -activeFleet.Y > maxExpectedDeviation {
						maxExpectedDeviation = -activeFleet.Y
					}
					if activeFleet.Y > maxWrongDeviation {
						maxWrongDeviation = activeFleet.Y
					}
				}
			}

			if !arrived {
				t.Fatal("expected fleet to reach the target")
			}

			if maxExpectedDeviation < 5 {
				t.Fatalf("expected visible avoidance on the short side, got %.2f", maxExpectedDeviation)
			}

			if maxExpectedDeviation <= maxWrongDeviation {
				t.Fatalf("expected shortest-side routing, expectedDeviation=%.2f wrongDeviation=%.2f", maxExpectedDeviation, maxWrongDeviation)
			}
		})
	}
}

func TestWinnerReturnsSoleRemainingOwner(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		planets: map[int]*Planet{
			1: {ID: 1, Owner: 1, Ships: 10},
			2: {ID: 2, Owner: 0, Ships: 5},
		},
		fleets: map[int]*Fleet{
			1: {ID: 1, Owner: 1, Ships: 3},
		},
	}

	winnerID, ok := engine.Winner()
	if !ok {
		t.Fatal("expected sole remaining owner to win")
	}
	if winnerID != 1 {
		t.Fatalf("expected winner 1, got %d", winnerID)
	}
}

func TestWinnerRequiresSingleRemainingOwner(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		planets: map[int]*Planet{
			1: {ID: 1, Owner: 1, Ships: 10},
			2: {ID: 2, Owner: 2, Ships: 10},
		},
		fleets: make(map[int]*Fleet),
	}

	if winnerID, ok := engine.Winner(); ok {
		t.Fatalf("expected no winner while multiple owners remain, got %d", winnerID)
	}
}

func TestSnapshotForPlayerIncludesAssignedColors(t *testing.T) {
	t.Parallel()

	engine := NewEngineWithConfig(MapConfig{PlayerCount: 3})

	snapshot := engine.SnapshotForPlayer(2)

	if len(snapshot.PlayerColors) != 3 {
		t.Fatalf("expected 3 player colors, got %d", len(snapshot.PlayerColors))
	}

	for index, color := range snapshot.PlayerColors {
		expectedPlayerID := index + 1
		if color.PlayerID != expectedPlayerID {
			t.Fatalf("expected player color entry %d to target player %d, got %d", index, expectedPlayerID, color.PlayerID)
		}
		if color.Color != playerColorPalette[index] {
			t.Fatalf("expected player %d color %#x, got %#x", expectedPlayerID, playerColorPalette[index], color.Color)
		}
	}
}
