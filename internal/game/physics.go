package game

import "math"

// Fleet movement uses a steering-force model: each tick a desired direction
// is computed from target pull, repulsion from nearby planets, and separation
// from nearby fleets. The heading then rotates at most fleetTurnRateRadians
// per second towards that desired direction.
const (
	targetPullAcceleration   = 360.0
	planetInfluencePadding   = 28.0
	planetRepulsionStrength  = 540.0
	planetTangentialStrength = 520.0
	fleetInfluencePadding    = 2.0
	fleetRepulsionStrength   = 90.0
	fleetTurnRateRadians     = 7.2
	fleetCollisionElasticity = 0.2
	fleetSeparationDistance  = 8.0
)

func (engine *Engine) advanceFleetLocked(id int, fleet *Fleet, target *Planet, steeringIndex *fleetSpatialIndex, planetIndex *planetSpatialIndex) {
	deltaTime := 1 / float64(engine.tickRate)
	desiredX, desiredY := engine.computeFleetAccelerationLocked(fleet, target, steeringIndex, planetIndex)
	desiredX, desiredY = normalizeVector(desiredX, desiredY)
	if desiredX == 0 && desiredY == 0 {
		desiredX, desiredY = normalizeVector(target.X-fleet.X, target.Y-fleet.Y)
	}

	currentX, currentY := normalizeVector(fleet.VX, fleet.VY)
	if currentX == 0 && currentY == 0 {
		currentX, currentY = desiredX, desiredY
	}

	nextHeadingX, nextHeadingY := rotateVectorTowards(
		currentX,
		currentY,
		desiredX,
		desiredY,
		fleetTurnRateRadians*deltaTime,
	)
	fleet.VX = nextHeadingX * engine.fleetSpeed
	fleet.VY = nextHeadingY * engine.fleetSpeed

	nextX := fleet.X + fleet.VX*deltaTime
	nextY := fleet.Y + fleet.VY*deltaTime
	if segmentIntersectsCircle(fleet.X, fleet.Y, nextX, nextY, target, fleetCollisionRadius) {
		engine.resolveArrivalLocked(id, fleet, target)
		return
	}

	fleet.X = nextX
	fleet.Y = nextY
	engine.resolvePlanetCollisionsLocked(fleet, target, planetIndex)
	fleet.VX, fleet.VY = clampMagnitude(fleet.VX, fleet.VY, engine.fleetSpeed)
}

func (engine *Engine) computeFleetAccelerationLocked(fleet *Fleet, target *Planet, steeringIndex *fleetSpatialIndex, planetIndex *planetSpatialIndex) (float64, float64) {
	targetX, targetY := normalizeVector(target.X-fleet.X, target.Y-fleet.Y)
	accelerationX := targetX * targetPullAcceleration
	accelerationY := targetY * targetPullAcceleration
	blocking := engine.currentAvoidancePlanetLocked(fleet, target, planetIndex)
	visitPlanet := func(planet *Planet) {
		if planet == nil {
			return
		}
		if planet.ID == target.ID {
			return
		}

		normalX, normalY, distance, ok := normalFromPoint(planet.X, planet.Y, fleet.X, fleet.Y)
		if !ok {
			return
		}

		influenceRadius := avoidanceRadius(planet) + planetInfluencePadding
		if distance >= influenceRadius {
			return
		}

		weight := clamp01((influenceRadius - distance) / planetInfluencePadding)
		accelerationX += normalX * planetRepulsionStrength * (0.25 + weight)
		accelerationY += normalY * planetRepulsionStrength * (0.25 + weight)

		if blocking != nil && planet.ID == blocking.ID && (segmentIntersectsCircle(fleet.X, fleet.Y, target.X, target.Y, planet, fleetCollisionRadius+avoidancePadding) || distance < avoidanceRadius(planet)+collisionPadding) {
			tangentX, tangentY := tangentVector(normalX, normalY, fleet.AvoidClockwise)
			accelerationX += tangentX * planetTangentialStrength * (0.2 + weight)
			accelerationY += tangentY * planetTangentialStrength * (0.2 + weight)
		}
	}

	if planetIndex != nil {
		planetIndex.forEachNearby(fleet.X, fleet.Y, planetIndex.maxInfluenceRadius, visitPlanet)
	} else {
		for _, planet := range engine.planets {
			visitPlanet(planet)
		}
	}

	influenceRadius := fleetSeparationDistance + fleetInfluencePadding
	if steeringIndex != nil {
		steeringIndex.forEachNearby(fleet.X, fleet.Y, influenceRadius, func(other *Fleet) {
			if other == nil || other.ID == fleet.ID {
				return
			}

			normalX, normalY, distance, ok := normalFromPoint(other.X, other.Y, fleet.X, fleet.Y)
			if !ok || distance >= influenceRadius {
				return
			}

			weight := clamp01((influenceRadius - distance) / fleetInfluencePadding)
			accelerationX += normalX * fleetRepulsionStrength * (0.2 + weight)
			accelerationY += normalY * fleetRepulsionStrength * (0.2 + weight)
		})
	}

	return accelerationX, accelerationY
}

func (engine *Engine) resolvePlanetCollisionsLocked(fleet *Fleet, target *Planet, planetIndex *planetSpatialIndex) {
	visitPlanet := func(planet *Planet) {
		if planet == nil {
			return
		}
		if planet.ID == target.ID {
			return
		}

		radius := avoidanceRadius(planet)
		normalX, normalY, distance, ok := normalFromPoint(planet.X, planet.Y, fleet.X, fleet.Y)
		if !ok {
			normalX, normalY = normalizeVector(-fleet.VY, fleet.VX)
			if normalX == 0 && normalY == 0 {
				normalX, normalY = 1, 0
			}
			distance = 0
		}

		if distance >= radius {
			return
		}

		fleet.X = planet.X + normalX*radius
		fleet.Y = planet.Y + normalY*radius

		inwardSpeed := fleet.VX*normalX + fleet.VY*normalY
		if inwardSpeed < 0 {
			fleet.VX -= inwardSpeed * normalX
			fleet.VY -= inwardSpeed * normalY
		}
	}

	if planetIndex != nil {
		planetIndex.forEachNearby(fleet.X, fleet.Y, planetIndex.maxCollisionRadius, visitPlanet)
	} else {
		for _, planet := range engine.planets {
			visitPlanet(planet)
		}
	}
}

func (engine *Engine) resolveFleetCollisionsLocked(collisionIndex *fleetSpatialIndex) {
	if len(engine.fleets) < 2 || collisionIndex == nil {
		return
	}

	for _, first := range engine.fleets {
		if first == nil {
			continue
		}

		collisionIndex.forEachNearby(first.X, first.Y, fleetSeparationDistance, func(second *Fleet) {
			if second == nil || second.ID <= first.ID {
				return
			}

			minimumDistance := fleetSeparationDistance
			normalX, normalY, distance, ok := normalFromPoint(first.X, first.Y, second.X, second.Y)
			if !ok {
				normalX, normalY = 1, 0
				distance = 0
			}

			if distance >= minimumDistance {
				return
			}

			overlap := minimumDistance - distance
			first.X -= normalX * overlap * 0.5
			first.Y -= normalY * overlap * 0.5
			second.X += normalX * overlap * 0.5
			second.Y += normalY * overlap * 0.5

			relVX := second.VX - first.VX
			relVY := second.VY - first.VY
			relNormalSpeed := relVX*normalX + relVY*normalY
			if relNormalSpeed >= 0 {
				return
			}

			impulse := -(1 + fleetCollisionElasticity) * relNormalSpeed * 0.5
			first.VX -= impulse * normalX
			first.VY -= impulse * normalY
			second.VX += impulse * normalX
			second.VY += impulse * normalY
			first.VX, first.VY = clampMagnitude(first.VX, first.VY, engine.fleetSpeed)
			second.VX, second.VY = clampMagnitude(second.VX, second.VY, engine.fleetSpeed)
		})
	}
}

func (engine *Engine) resolveArrivalLocked(id int, fleet *Fleet, target *Planet) {
	fleet.X = target.X
	fleet.Y = target.Y

	if target.Owner == fleet.Owner {
		target.Ships += fleet.Ships
	} else {
		target.Ships -= fleet.Ships
		if target.Ships < 0 {
			target.Owner = fleet.Owner
			target.Ships = -target.Ships
		}
	}

	delete(engine.fleets, id)
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}

	return value
}

func clampMagnitude(x, y, maxMag float64) (float64, float64) {
	length := math.Hypot(x, y)
	if length == 0 || length <= maxMag {
		return x, y
	}

	scale := maxMag / length
	return x * scale, y * scale
}

func rotateVectorTowards(currentX, currentY, targetX, targetY, maxTurn float64) (float64, float64) {
	currentX, currentY = normalizeVector(currentX, currentY)
	targetX, targetY = normalizeVector(targetX, targetY)
	if targetX == 0 && targetY == 0 {
		return currentX, currentY
	}
	if currentX == 0 && currentY == 0 {
		return targetX, targetY
	}

	currentAngle := math.Atan2(currentY, currentX)
	targetAngle := math.Atan2(targetY, targetX)
	deltaAngle := shortestSignedAngle(currentAngle, targetAngle)
	if deltaAngle > maxTurn {
		deltaAngle = maxTurn
	} else if deltaAngle < -maxTurn {
		deltaAngle = -maxTurn
	}

	nextAngle := currentAngle + deltaAngle
	return math.Cos(nextAngle), math.Sin(nextAngle)
}

func shortestSignedAngle(from, to float64) float64 {
	delta := math.Mod(to-from+math.Pi, 2*math.Pi) - math.Pi
	if delta < -math.Pi {
		return delta + 2*math.Pi
	}
	return delta
}

func normalFromPoint(originX, originY, pointX, pointY float64) (float64, float64, float64, bool) {
	dx := pointX - originX
	dy := pointY - originY
	distance := math.Hypot(dx, dy)
	if distance == 0 {
		return 0, 0, 0, false
	}

	return dx / distance, dy / distance, distance, true
}
