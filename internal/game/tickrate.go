package game

import "math"

// resolveDynamicTickRateLocked raises the simulation tick rate from idle to
// full whenever any fleet is close enough to its target or a planet obstacle
// that a coarse time step could cause it to tunnel through a collision
// boundary. This keeps the simulation cheap during calm moments while
// preserving accuracy near collisions.
func (engine *Engine) resolveDynamicTickRateLocked() int {
	if len(engine.fleets) == 0 {
		return DefaultIdleTickRate
	}

	planetIndex := engine.planetIndex
	if planetIndex == nil {
		planetIndex = newPlanetSpatialIndex(engine.planets)
		engine.planetIndex = planetIndex
	}

	slowStepSeconds := 1 / float64(DefaultIdleTickRate)
	riskDistance := engine.fleetSpeed * slowStepSeconds
	fleetIndex := newFleetSpatialIndex(engine.fleets, riskDistance+fleetSeparationDistance)

	for _, fleet := range engine.fleets {
		if fleet == nil {
			continue
		}

		target := engine.planets[fleet.TargetID]
		if target == nil {
			continue
		}

		nextX := fleet.X + fleet.VX*slowStepSeconds
		nextY := fleet.Y + fleet.VY*slowStepSeconds
		if segmentIntersectsCircle(fleet.X, fleet.Y, nextX, nextY, target, fleetCollisionRadius) {
			return DefaultTickRate
		}

		planetRisk := false
		planetSearchRadius := riskDistance + planetIndex.maxCollisionRadius
		planetIndex.forEachNearby(fleet.X, fleet.Y, planetSearchRadius, func(planet *Planet) {
			if planetRisk || planet == nil || planet.ID == target.ID {
				return
			}

			if segmentIntersectsCircle(fleet.X, fleet.Y, nextX, nextY, planet, fleetCollisionRadius+collisionPadding) {
				planetRisk = true
				return
			}

			distance := math.Hypot(fleet.X-planet.X, fleet.Y-planet.Y)
			if distance < avoidanceRadius(planet)+planetInfluencePadding+riskDistance {
				planetRisk = true
			}
		})
		if planetRisk {
			return DefaultTickRate
		}

		fleetRisk := false
		fleetIndex.forEachNearby(fleet.X, fleet.Y, riskDistance+fleetSeparationDistance, func(other *Fleet) {
			if fleetRisk || other == nil || other.ID == fleet.ID {
				return
			}

			nextOtherX := other.X + other.VX*slowStepSeconds
			nextOtherY := other.Y + other.VY*slowStepSeconds
			if distanceSquared(nextX, nextY, nextOtherX, nextOtherY) < fleetSeparationDistance*fleetSeparationDistance*4 {
				fleetRisk = true
				return
			}

			if distanceSquared(fleet.X, fleet.Y, other.X, other.Y) < (fleetSeparationDistance+riskDistance)*(fleetSeparationDistance+riskDistance) {
				fleetRisk = true
			}
		})
		if fleetRisk {
			return DefaultTickRate
		}
	}

	return DefaultIdleTickRate
}
