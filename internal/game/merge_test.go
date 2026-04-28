package game

import "testing"

func fleetsWithMergePadding(active map[int]*Fleet) map[int]*Fleet {
	padded := make(map[int]*Fleet, fleetMergeActivationStep)
	for id, fleet := range active {
		copyFleet := *fleet
		padded[id] = &copyFleet
	}

	nextID := 1000
	for len(padded) < fleetMergeActivationStep {
		padded[nextID] = &Fleet{
			ID:       nextID,
			Owner:    nextID,
			SourceID: nextID,
			TargetID: nextID + 1,
			Ships:    1,
			X:        float64(nextID * 100),
			Y:        float64(nextID * 100),
			VX:       80,
			VY:       0,
		}
		nextID++
	}

	return padded
}

func TestMergeFleetsLockedCoalescesTightMatchingGroup(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		fleets: fleetsWithMergePadding(map[int]*Fleet{
			1: {ID: 1, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 100, Y: 100, VX: 80, VY: 0},
			2: {ID: 2, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 108, Y: 101, VX: 79, VY: 2},
		}),
	}

	mergeIndex := newFleetSpatialIndex(engine.fleets, fleetMergeDistance)
	engine.mergeFleetsLocked(mergeIndex)

	if len(engine.fleets) != fleetMergeActivationStep-1 {
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
	t.Parallel()

	engine := &Engine{
		fleets: fleetsWithMergePadding(map[int]*Fleet{
			1: {ID: 1, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 100, Y: 100, VX: 80, VY: 0},
			2: {ID: 2, Owner: 1, SourceID: 10, TargetID: 21, Ships: 1, X: 108, Y: 101, VX: 80, VY: 0},
		}),
	}

	mergeIndex := newFleetSpatialIndex(engine.fleets, fleetMergeDistance)
	engine.mergeFleetsLocked(mergeIndex)

	if len(engine.fleets) != fleetMergeActivationStep {
		t.Fatalf("expected fleets with different targets to stay separate, got %d", len(engine.fleets))
	}
}

func TestMergeFleetsLockedStopsAfterSmallBundle(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		fleets: fleetsWithMergePadding(map[int]*Fleet{
			1: {ID: 1, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 100, Y: 100, VX: 80, VY: 0},
			2: {ID: 2, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 108, Y: 101, VX: 79, VY: 2},
			3: {ID: 3, Owner: 1, SourceID: 10, TargetID: 20, Ships: 1, X: 104, Y: 99, VX: 80, VY: 1},
		}),
	}

	mergeIndex := newFleetSpatialIndex(engine.fleets, fleetMergeDistance)
	engine.mergeFleetsLocked(mergeIndex)

	if len(engine.fleets) != fleetMergeActivationStep-1 {
		t.Fatalf("expected one small merge but not full collapse, got %d fleets", len(engine.fleets))
	}

	mergedCount := 0
	singleCount := 0
	for _, fleetID := range []int{1, 2, 3} {
		fleet := engine.fleets[fleetID]
		if fleet == nil {
			continue
		}

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
	t.Parallel()

	if mergeMaxShips := dynamicFleetMergeMaxShips(fleetMergeActivationStep - 1); mergeMaxShips != 1 {
		t.Fatalf("expected merging to stay disabled below %d fleets, got %d", fleetMergeActivationStep, mergeMaxShips)
	}

	if mergeMaxShips := dynamicFleetMergeMaxShips(fleetMergeActivationStep); mergeMaxShips != baseFleetMergeMaxShips {
		t.Fatalf("expected merge bundles to activate at %d fleets with size %d, got %d", fleetMergeActivationStep, baseFleetMergeMaxShips, mergeMaxShips)
	}

	expectedMidScale := baseFleetMergeMaxShips + 2500/fleetMergeScaleStep
	if mergeMaxShips := dynamicFleetMergeMaxShips(2500); mergeMaxShips != expectedMidScale {
		t.Fatalf("expected larger fleet counts to allow larger bundles, got %d", mergeMaxShips)
	}

	if mergeMaxShips := dynamicFleetMergeMaxShips(24000); mergeMaxShips != maxFleetMergeMaxShips {
		t.Fatalf("expected merge bundle size to clamp at %d, got %d", maxFleetMergeMaxShips, mergeMaxShips)
	}
}

func TestLaunchFleetBundleSizeWaitsForMergeThreshold(t *testing.T) {
	t.Parallel()

	if bundleSize := launchFleetBundleSize(0, fleetMergeActivationStep-1); bundleSize != 1 {
		t.Fatalf("expected launch bundling to stay disabled below %d fleets, got %d", fleetMergeActivationStep, bundleSize)
	}

	if bundleSize := launchFleetBundleSize(0, fleetMergeActivationStep); bundleSize != baseFleetMergeMaxShips {
		t.Fatalf("expected launch bundling to activate at %d fleets with size %d, got %d", fleetMergeActivationStep, baseFleetMergeMaxShips, bundleSize)
	}
}
