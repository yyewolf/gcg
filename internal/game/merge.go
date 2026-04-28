package game

import "math"

// Fleet merging coalesces nearby same-owner same-route fleets into a single
// bundle to keep the active fleet count manageable at scale. Merging is
// disabled below fleetMergeActivationStep total fleets to avoid overhead
// when the feature isn't needed.
const (
	fleetMergeDistance       = 12.0
	fleetMergeHeadingDot     = 0.985
	fleetMergeActivationStep = 500
	baseFleetMergeMaxShips   = 2
	maxFleetMergeMaxShips    = 32
	fleetMergeScaleStep      = 600
)

func (engine *Engine) mergeFleets(mergeIndex *fleetSpatialIndex) {
	if len(engine.fleets) < 2 || mergeIndex == nil {
		return
	}

	mergeMaxShips := dynamicFleetMergeMaxShips(len(engine.fleets))

	for _, first := range engine.fleets {
		if first == nil {
			continue
		}

		mergeIndex.forEachNearby(first.X, first.Y, fleetMergeDistance, func(second *Fleet) {
			if second == nil || second.ID <= first.ID {
				return
			}
			if engine.fleets[first.ID] != first || engine.fleets[second.ID] != second {
				return
			}
			if !canMergeFleets(first, second, mergeMaxShips) {
				return
			}

			mergeFleet(first, second)
			engine.removeSortedFleetID(second.ID)
			delete(engine.fleets, second.ID)
		})
	}
}

// dynamicFleetMergeMaxShips returns the maximum ships allowed in one bundle
// at the given fleet count. The limit grows with fleet count so large battles
// keep things manageable without over-merging during normal play.
func dynamicFleetMergeMaxShips(fleetCount int) int {
	if fleetCount < fleetMergeActivationStep {
		return 1
	}

	mergeMaxShips := baseFleetMergeMaxShips + fleetCount/fleetMergeScaleStep
	if mergeMaxShips > maxFleetMergeMaxShips {
		return maxFleetMergeMaxShips
	}

	return mergeMaxShips
}

func canMergeFleets(first, second *Fleet, mergeMaxShips int) bool {
	if first.Owner != second.Owner || first.SourceID != second.SourceID || first.TargetID != second.TargetID {
		return false
	}
	if first.Ships+second.Ships > mergeMaxShips {
		return false
	}
	if first.AvoidPlanetID != second.AvoidPlanetID || first.AvoidClockwise != second.AvoidClockwise {
		return false
	}
	if distanceSquared(first.X, first.Y, second.X, second.Y) > fleetMergeDistance*fleetMergeDistance {
		return false
	}

	firstHeadingX, firstHeadingY := normalizeVector(first.VX, first.VY)
	secondHeadingX, secondHeadingY := normalizeVector(second.VX, second.VY)
	if firstHeadingX == 0 && firstHeadingY == 0 {
		return false
	}
	if secondHeadingX == 0 && secondHeadingY == 0 {
		return false
	}

	return firstHeadingX*secondHeadingX+firstHeadingY*secondHeadingY >= fleetMergeHeadingDot
}

// mergeFleet absorbs secondary into primary, blending position and velocity
// by ship count as weights.
func mergeFleet(primary, secondary *Fleet) {
	totalShips := primary.Ships + secondary.Ships
	if totalShips <= 0 {
		return
	}

	primary.X = weightedAverage(primary.X, float64(primary.Ships), secondary.X, float64(secondary.Ships))
	primary.Y = weightedAverage(primary.Y, float64(primary.Ships), secondary.Y, float64(secondary.Ships))
	primary.VX = weightedAverage(primary.VX, float64(primary.Ships), secondary.VX, float64(secondary.Ships))
	primary.VY = weightedAverage(primary.VY, float64(primary.Ships), secondary.VY, float64(secondary.Ships))
	primary.Ships = totalShips
	if secondary.LaunchTick < primary.LaunchTick {
		primary.LaunchTick = secondary.LaunchTick
	}
	if secondary.ETA < primary.ETA || primary.ETA == 0 {
		primary.ETA = secondary.ETA
	}
	primary.VX, primary.VY = clampMagnitude(primary.VX, primary.VY, math.Hypot(primary.VX, primary.VY))
}

func weightedAverage(firstValue, firstWeight, secondValue, secondWeight float64) float64 {
	weight := firstWeight + secondWeight
	if weight == 0 {
		return firstValue
	}

	return (firstValue*firstWeight + secondValue*secondWeight) / weight
}
