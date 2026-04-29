package game

import "math"

// Spatial indices bucket world objects into fixed-size grid cells so that
// proximity queries scan only the cells overlapping a given radius instead
// of iterating every object in the world.

// fleetSpatialCell is a packed (cellX, cellY) grid coordinate encoded as a
// single int64 so the runtime uses mapaccess1_fast64 instead of the generic
// struct-key path (memhash128 / memequal128), which was ~50% of CPU time.
type fleetSpatialCell = int64

// packCell encodes a (cellX, cellY) pair into an int64. Each axis is stored
// as a uint32, so coordinates in [-2^31, 2^31-1] are represented exactly.
func packCell(x, y int) fleetSpatialCell {
	return int64(uint32(x))<<32 | int64(uint32(y))
}

type fleetSpatialIndex struct {
	cellSize  float64
	cells     map[fleetSpatialCell][]*Fleet
	dirtyKeys []fleetSpatialCell
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

// reset clears the index and rebuilds it from fleets, reusing the cells map
// and per-cell slice backing arrays to avoid per-tick allocations.
// Only cells that were populated in the previous tick are zeroed; the rest of
// the map is left untouched (they already hold nil/empty slices).
func (index *fleetSpatialIndex) reset(fleets map[int]*Fleet, cellSize float64) {
	if cellSize <= 0 {
		cellSize = 1
	}
	index.cellSize = cellSize

	// Lazy initialization on first use.
	if index.cells == nil {
		index.cells = make(map[fleetSpatialCell][]*Fleet, len(fleets))
	}

	// Zero bucket lengths for all cells touched in the previous tick.
	// This retains the backing arrays so future appends are allocation-free.
	for _, k := range index.dirtyKeys {
		index.cells[k] = index.cells[k][:0]
	}
	index.dirtyKeys = index.dirtyKeys[:0]

	// Repopulate.
	for _, fleet := range fleets {
		if fleet == nil {
			continue
		}
		cell := index.cellFor(fleet.X, fleet.Y)
		if len(index.cells[cell]) == 0 {
			// First write to this cell this tick — track it for the next reset.
			index.dirtyKeys = append(index.dirtyKeys, cell)
		}
		index.cells[cell] = append(index.cells[cell], fleet)
	}
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
			for _, fleet := range index.cells[packCell(cellX, cellY)] {
				visit(fleet)
			}
		}
	}
}

func (index *fleetSpatialIndex) cellFor(x, y float64) fleetSpatialCell {
	return packCell(int(math.Floor(x/index.cellSize)), int(math.Floor(y/index.cellSize)))
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
			for _, planet := range index.cells[packCell(cellX, cellY)] {
				visit(planet)
			}
		}
	}
}

func (index *planetSpatialIndex) cellFor(x, y float64) fleetSpatialCell {
	return packCell(int(math.Floor(x/index.cellSize)), int(math.Floor(y/index.cellSize)))
}
