package game

import (
	"math"
	"testing"
)

func TestSendFleetSpawnsShipsAroundSourcePlanet(t *testing.T) {
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

	naiveETA := fleet.ETA
	maxAbsY := 0.0
	arrived := false
	minimumClearance := avoidanceRadius(engine.planets[3]) - 0.001

	for step := 0; step < 180; step++ {
		engine.Advance()

		activeFleet, ok := engine.fleets[fleet.ID]
		if !ok {
			arrived = true
			if engine.tick <= naiveETA {
				t.Fatalf("fleet arrived too early after collision handling: tick=%d naiveETA=%d", engine.tick, naiveETA)
			}
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

			for step := 0; step < 180; step++ {
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
