package game

import "math"

// Spatial indices bucket world objects into fixed-size grid cells so that
// proximity queries scan only the cells overlapping a given radius instead
// of iterating every object in the world.

type fleetSpatialCell struct {
	x int
	y int
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
