package game

import "math"

const (
	targetPullAcceleration   = 360.0
	planetInfluencePadding   = 28.0
	planetRepulsionStrength  = 540.0
	planetTangentialStrength = 520.0
	avoidanceSwitchMargin    = 12.0
	fleetInfluencePadding    = 2.0
	fleetRepulsionStrength   = 90.0
	fleetTurnRateRadians     = 7.2
	fleetMergeDistance       = 12.0
	fleetMergeHeadingDot     = 0.985
	fleetMergeActivationStep = 500
	baseFleetMergeMaxShips   = 2
	maxFleetMergeMaxShips    = 32
	fleetMergeScaleStep      = 600
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

func (engine *Engine) mergeFleetsLocked(mergeIndex *fleetSpatialIndex) {
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
			delete(engine.fleets, second.ID)
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

func dynamicFleetMergeMaxShips(fleetCount int) int {
	if fleetCount < fleetMergeActivationStep {
		return 1
	}

	mergeMaxShips := baseFleetMergeMaxShips
	if fleetCount > 0 {
		mergeMaxShips += fleetCount / fleetMergeScaleStep
	}
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

func avoidanceRadius(planet *Planet) float64 {
	return planet.Radius + fleetCollisionRadius + collisionPadding
}

func (engine *Engine) currentAvoidancePlanetLocked(fleet *Fleet, target *Planet, planetIndex *planetSpatialIndex) *Planet {
	if current := engine.planets[fleet.AvoidPlanetID]; current != nil && engine.shouldKeepAvoidingPlanetLocked(fleet, target, current) {
		return current
	}

	blocking := engine.findBlockingPlanetLocked(fleet, target, planetIndex)
	if blocking == nil {
		fleet.AvoidPlanetID = 0
		fleet.AvoidClockwise = false
		return nil
	}

	fleet.AvoidClockwise = engine.chooseAvoidanceClockwiseLocked(fleet, blocking, target)
	fleet.AvoidPlanetID = blocking.ID
	return blocking
}

func (engine *Engine) shouldKeepAvoidingPlanetLocked(fleet *Fleet, target *Planet, obstacle *Planet) bool {
	if segmentIntersectsCircle(fleet.X, fleet.Y, target.X, target.Y, obstacle, fleetCollisionRadius+avoidancePadding) {
		return true
	}

	return math.Hypot(fleet.X-obstacle.X, fleet.Y-obstacle.Y) < avoidanceRadius(obstacle)+planetInfluencePadding
}

func (engine *Engine) findBlockingPlanetLocked(fleet *Fleet, target *Planet, planetIndex *planetSpatialIndex) *Planet {
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

func (engine *Engine) chooseAvoidanceClockwiseLocked(fleet *Fleet, obstacle, target *Planet) bool {
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

		var directionX float64
		var directionY float64
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

type avoidanceRoute struct {
	totalLength float64
}

type fleetSpatialIndex struct {
	cellSize float64
	cells    map[fleetSpatialCell][]*Fleet
}

type planetSpatialIndex struct {
	cellSize           float64
	maxBlockingRadius  float64
	maxCollisionRadius float64
	maxInfluenceRadius float64
	cells              map[fleetSpatialCell][]*Planet
}

type fleetSpatialCell struct {
	x int
	y int
}

func newPlanetSpatialIndex(planets map[int]*Planet) *planetSpatialIndex {
	maxBlockingRadius := 1.0
	maxCollisionRadius := 1.0
	maxInfluenceRadius := 1.0
	for _, planet := range planets {
		if planet == nil {
			continue
		}

		blockingRadius := planet.Radius + fleetCollisionRadius + avoidancePadding
		collisionRadius := avoidanceRadius(planet)
		influenceRadius := collisionRadius + planetInfluencePadding
		if blockingRadius > maxBlockingRadius {
			maxBlockingRadius = blockingRadius
		}
		if collisionRadius > maxCollisionRadius {
			maxCollisionRadius = collisionRadius
		}
		if influenceRadius > maxInfluenceRadius {
			maxInfluenceRadius = influenceRadius
		}
	}

	index := &planetSpatialIndex{
		cellSize:           maxInfluenceRadius,
		maxBlockingRadius:  maxBlockingRadius,
		maxCollisionRadius: maxCollisionRadius,
		maxInfluenceRadius: maxInfluenceRadius,
		cells:              make(map[fleetSpatialCell][]*Planet, len(planets)),
	}

	for _, planet := range planets {
		if planet == nil {
			continue
		}

		cell := index.cellFor(planet.X, planet.Y)
		index.cells[cell] = append(index.cells[cell], planet)
	}

	return index
}

func newFleetSpatialIndex(fleets map[int]*Fleet, cellSize float64) *fleetSpatialIndex {
	if cellSize <= 0 {
		cellSize = 1
	}

	index := &fleetSpatialIndex{
		cellSize: cellSize,
		cells:    make(map[fleetSpatialCell][]*Fleet, len(fleets)),
	}

	for _, fleet := range fleets {
		if fleet == nil {
			continue
		}

		cell := index.cellFor(fleet.X, fleet.Y)
		index.cells[cell] = append(index.cells[cell], fleet)
	}

	return index
}

func (index *fleetSpatialIndex) forEachNearby(x, y, radius float64, visit func(*Fleet)) {
	if index == nil {
		return
	}

	minCellX := int(math.Floor((x - radius) / index.cellSize))
	maxCellX := int(math.Floor((x + radius) / index.cellSize))
	minCellY := int(math.Floor((y - radius) / index.cellSize))
	maxCellY := int(math.Floor((y + radius) / index.cellSize))

	for cellX := minCellX; cellX <= maxCellX; cellX++ {
		for cellY := minCellY; cellY <= maxCellY; cellY++ {
			for _, fleet := range index.cells[fleetSpatialCell{x: cellX, y: cellY}] {
				visit(fleet)
			}
		}
	}
}

func (index *fleetSpatialIndex) cellFor(x, y float64) fleetSpatialCell {
	return fleetSpatialCell{
		x: int(math.Floor(x / index.cellSize)),
		y: int(math.Floor(y / index.cellSize)),
	}
}

func (index *planetSpatialIndex) forEachNearby(x, y, radius float64, visit func(*Planet)) {
	if index == nil {
		return
	}

	index.forEachInBounds(x-radius, x+radius, y-radius, y+radius, visit)
}

func (index *planetSpatialIndex) forEachInBounds(minX, maxX, minY, maxY float64, visit func(*Planet)) {
	if index == nil {
		return
	}

	minCellX := int(math.Floor(minX / index.cellSize))
	maxCellX := int(math.Floor(maxX / index.cellSize))
	minCellY := int(math.Floor(minY / index.cellSize))
	maxCellY := int(math.Floor(maxY / index.cellSize))

	for cellX := minCellX; cellX <= maxCellX; cellX++ {
		for cellY := minCellY; cellY <= maxCellY; cellY++ {
			for _, planet := range index.cells[fleetSpatialCell{x: cellX, y: cellY}] {
				visit(planet)
			}
		}
	}
}

func (index *planetSpatialIndex) cellFor(x, y float64) fleetSpatialCell {
	return fleetSpatialCell{
		x: int(math.Floor(x / index.cellSize)),
		y: int(math.Floor(y / index.cellSize)),
	}
}
