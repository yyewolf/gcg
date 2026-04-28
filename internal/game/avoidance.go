package game

import "math"

// Planet avoidance uses tangent-line routing: when a planet blocks the direct
// path to the target, the fleet steers towards the tangent point on the
// planet's clearance circle, choosing the shorter arc. A hysteresis margin
// (avoidanceSwitchMargin) prevents rapid side-switching.
const avoidanceSwitchMargin = 12.0

// avoidanceRoute holds the estimated path length around a planet obstacle.
type avoidanceRoute struct {
	totalLength float64
}

// avoidanceRadius returns the minimum clearance distance a fleet must maintain
// from a planet's center (planet radius + fleet body + collision buffer).
func avoidanceRadius(planet *Planet) float64 {
	return planet.Radius + fleetCollisionRadius + collisionPadding
}

// currentAvoidancePlanet returns the planet the fleet is currently routing
// around, or nil if the direct path is clear. It refreshes the avoidance
// target whenever the old one is no longer blocking.
func (engine *Engine) currentAvoidancePlanet(fleet *Fleet, target *Planet, planetIndex *planetSpatialIndex) *Planet {
	if current := engine.planets[fleet.AvoidPlanetID]; current != nil && engine.shouldKeepAvoidingPlanet(fleet, target, current) {
		return current
	}

	blocking := engine.findBlockingPlanet(fleet, target, planetIndex)
	if blocking == nil {
		fleet.AvoidPlanetID = 0
		fleet.AvoidClockwise = false
		return nil
	}

	fleet.AvoidClockwise = engine.chooseAvoidanceClockwise(fleet, blocking, target)
	fleet.AvoidPlanetID = blocking.ID
	return blocking
}

func (engine *Engine) shouldKeepAvoidingPlanet(fleet *Fleet, target *Planet, obstacle *Planet) bool {
	if segmentIntersectsCircle(fleet.X, fleet.Y, target.X, target.Y, obstacle, fleetCollisionRadius+avoidancePadding) {
		return true
	}

	return math.Hypot(fleet.X-obstacle.X, fleet.Y-obstacle.Y) < avoidanceRadius(obstacle)+planetInfluencePadding
}

// findBlockingPlanet returns the nearest planet that intersects the direct
// line from the fleet to its target, or nil if the path is clear.
func (engine *Engine) findBlockingPlanet(fleet *Fleet, target *Planet, planetIndex *planetSpatialIndex) *Planet {
	var blocking *Planet
	bestDistance := math.MaxFloat64
	visitPlanet := func(planet *Planet) {
		if planet == nil {
			return
		}

		if planet.ID == fleet.SourceID || planet.ID == fleet.TargetID {
			return
		}

		if !segmentIntersectsCircle(fleet.X, fleet.Y, target.X, target.Y, planet, fleetCollisionRadius+avoidancePadding) {
			return
		}

		distance := math.Hypot(planet.X-fleet.X, planet.Y-fleet.Y)
		if distance >= bestDistance {
			return
		}

		blocking = planet
		bestDistance = distance
	}

	if planetIndex != nil {
		padding := planetIndex.maxBlockingRadius
		minX := math.Min(fleet.X, target.X) - padding
		maxX := math.Max(fleet.X, target.X) + padding
		minY := math.Min(fleet.Y, target.Y) - padding
		maxY := math.Max(fleet.Y, target.Y) + padding
		planetIndex.forEachInBounds(minX, maxX, minY, maxY, visitPlanet)
	} else {
		for _, planet := range engine.planets {
			visitPlanet(planet)
		}
	}

	return blocking
}

// chooseAvoidanceClockwise picks the shorter tangent arc around the
// obstacle. If the fleet is already committed to one side, it stays on that
// side unless the other is shorter by more than avoidanceSwitchMargin.
func (engine *Engine) chooseAvoidanceClockwise(fleet *Fleet, obstacle, target *Planet) bool {
	clockwiseRoute, clockwiseOK := buildAvoidanceRoute(fleet.X, fleet.Y, obstacle, target.X, target.Y, true)
	counterRoute, counterOK := buildAvoidanceRoute(fleet.X, fleet.Y, obstacle, target.X, target.Y, false)
	if !clockwiseOK {
		return false
	}
	if !counterOK {
		return true
	}

	if fleet.AvoidPlanetID == obstacle.ID {
		if fleet.AvoidClockwise && clockwiseRoute.totalLength <= counterRoute.totalLength+avoidanceSwitchMargin {
			return true
		}
		if !fleet.AvoidClockwise && counterRoute.totalLength <= clockwiseRoute.totalLength+avoidanceSwitchMargin {
			return false
		}
	}

	return clockwiseRoute.totalLength <= counterRoute.totalLength
}

// buildAvoidanceRoute computes the total tangent path length (entry segment +
// arc + exit segment) around an obstacle in the given direction.
func buildAvoidanceRoute(startX, startY float64, obstacle *Planet, targetX, targetY float64, clockwise bool) (avoidanceRoute, bool) {
	clearance := avoidanceRadius(obstacle)
	entryAngle, ok := tangentAngleForEntry(startX, startY, obstacle, clearance, clockwise)
	if !ok {
		return avoidanceRoute{}, false
	}

	exitAngle, ok := tangentAngleForExit(targetX, targetY, obstacle, clearance, clockwise)
	if !ok {
		return avoidanceRoute{}, false
	}

	entryX, entryY := pointOnCircle(obstacle, clearance, entryAngle)
	exitX, exitY := pointOnCircle(obstacle, clearance, exitAngle)
	entryDistance := math.Hypot(entryX-startX, entryY-startY)
	arcDistance := angularDistance(entryAngle, exitAngle, clockwise) * clearance
	exitDistance := math.Hypot(targetX-exitX, targetY-exitY)

	return avoidanceRoute{totalLength: entryDistance + arcDistance + exitDistance}, true
}

func pointOnCircle(planet *Planet, radius, angle float64) (float64, float64) {
	return planet.X + math.Cos(angle)*radius, planet.Y + math.Sin(angle)*radius
}

func tangentAngleForEntry(pointX, pointY float64, obstacle *Planet, radius float64, clockwise bool) (float64, bool) {
	return tangentAngleForPoint(pointX, pointY, obstacle, radius, clockwise, false)
}

func tangentAngleForExit(pointX, pointY float64, obstacle *Planet, radius float64, clockwise bool) (float64, bool) {
	return tangentAngleForPoint(pointX, pointY, obstacle, radius, clockwise, true)
}

func tangentAngleForPoint(pointX, pointY float64, obstacle *Planet, radius float64, clockwise bool, outgoing bool) (float64, bool) {
	dx := pointX - obstacle.X
	dy := pointY - obstacle.Y
	distance := math.Hypot(dx, dy)
	if distance <= radius {
		return 0, false
	}

	pointAngle := math.Atan2(dy, dx)
	offset := math.Acos(radius / distance)
	candidates := [2]float64{
		normalizeAngle(pointAngle + offset),
		normalizeAngle(pointAngle - offset),
	}

	bestAngle := candidates[0]
	bestScore := math.Inf(-1)
	for _, candidate := range candidates {
		tangentX, tangentY := tangentVector(math.Cos(candidate), math.Sin(candidate), clockwise)
		edgeX, edgeY := pointOnCircle(obstacle, radius, candidate)

		var directionX, directionY float64
		if outgoing {
			directionX, directionY = normalizeVector(pointX-edgeX, pointY-edgeY)
		} else {
			directionX, directionY = normalizeVector(edgeX-pointX, edgeY-pointY)
		}

		score := tangentX*directionX + tangentY*directionY
		if score > bestScore {
			bestScore = score
			bestAngle = candidate
		}
	}

	return bestAngle, true
}

func angularDistance(start, end float64, clockwise bool) float64 {
	start = normalizeAngle(start)
	end = normalizeAngle(end)
	if clockwise {
		if start >= end {
			return start - end
		}
		return start + 2*math.Pi - end
	}

	if end >= start {
		return end - start
	}
	return end + 2*math.Pi - start
}

func normalizeAngle(angle float64) float64 {
	wrapped := math.Mod(angle, 2*math.Pi)
	if wrapped < 0 {
		wrapped += 2 * math.Pi
	}
	return wrapped
}
