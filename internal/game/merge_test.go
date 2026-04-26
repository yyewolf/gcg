package game

import "testing"

func TestMergeFleetsLockedCoalescesTightMatchingGroup(t *testing.T) {
	engine := &Engine{
		fleets: map[int]*Fleet{
			1: {ID: 1, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 100, Y: 100, VX: 80, VY: 0},
			2: {ID: 2, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 108, Y: 101, VX: 79, VY: 2},
		},
	}

	mergeIndex := newFleetSpatialIndex(engine.fleets, fleetMergeDistance)
	engine.mergeFleetsLocked(mergeIndex)

	if len(engine.fleets) != 1 {
		t.Fatalf("expected fleets to merge into one backend fleet, got %d", len(engine.fleets))
	}

	merged := engine.fleets[1]
	if merged == nil {
		t.Fatal("expected primary fleet to remain after merge")
	}
	if merged.Ships != 2 {
		t.Fatalf("expected merged fleet to carry 2 ships, got %d", merged.Ships)
	}
}

func TestMergeFleetsLockedKeepsDifferentRoutesSeparate(t *testing.T) {
	engine := &Engine{
		fleets: map[int]*Fleet{
			1: {ID: 1, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 100, Y: 100, VX: 80, VY: 0},
			2: {ID: 2, Owner: 1, SourceID: 10, TargetID: 21, Ships: 1, X: 108, Y: 101, VX: 80, VY: 0},
		},
	}

	mergeIndex := newFleetSpatialIndex(engine.fleets, fleetMergeDistance)
	engine.mergeFleetsLocked(mergeIndex)

	if len(engine.fleets) != 2 {
		t.Fatalf("expected fleets with different targets to stay separate, got %d", len(engine.fleets))
	}
}

func TestMergeFleetsLockedStopsAfterSmallBundle(t *testing.T) {
	engine := &Engine{
		fleets: map[int]*Fleet{
			1: {ID: 1, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 100, Y: 100, VX: 80, VY: 0},
			2: {ID: 2, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 108, Y: 101, VX: 79, VY: 2},
			3: {ID: 3, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 104, Y: 99, VX: 80, VY: 1},
		},
	}

	mergeIndex := newFleetSpatialIndex(engine.fleets, fleetMergeDistance)
	engine.mergeFleetsLocked(mergeIndex)

	if len(engine.fleets) != 2 {
		t.Fatalf("expected one small merge but not full collapse, got %d fleets", len(engine.fleets))
	}

	mergedCount := 0
	singleCount := 0
	for _, fleet := range engine.fleets {
		switch fleet.Ships {
		case 2:
			mergedCount++
		case 1:
			singleCount++
		}
	}

	if mergedCount != 1 || singleCount != 1 {
		t.Fatalf("expected one 2-ship bundle and one single ship, got merged=%d singles=%d", mergedCount, singleCount)
	}
}

func TestDynamicFleetMergeMaxShipsScalesWithFleetCount(t *testing.T) {
	if mergeMaxShips := dynamicFleetMergeMaxShips(300); mergeMaxShips != 2 {
		t.Fatalf("expected low fleet counts to keep small merge bundles, got %d", mergeMaxShips)
	}

	expectedMidScale := baseFleetMergeMaxShips + 2500/fleetMergeScaleStep
	if mergeMaxShips := dynamicFleetMergeMaxShips(2500); mergeMaxShips != expectedMidScale {
		t.Fatalf("expected larger fleet counts to allow larger bundles, got %d", mergeMaxShips)
	}

	if mergeMaxShips := dynamicFleetMergeMaxShips(12000); mergeMaxShips != maxFleetMergeMaxShips {
		t.Fatalf("expected merge bundle size to clamp at %d, got %d", maxFleetMergeMaxShips, mergeMaxShips)
	}
}
