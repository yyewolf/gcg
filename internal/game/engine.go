package game

import (
	"math"
	"runtime"
	"slices"
	"sync"
)

const shipLaunchSpacing = 1.5

const launchPositionTolerance = 0.001

const planetGrowthIntervalSeconds = 2
const growthTimeEpsilon = 1e-9

var playerColorPalette = []int{
	0x63f0ff,
	0xff728c,
	0x8bff6a,
	0xffd166,
	0xc792ff,
	0xff9f5c,
	0x5eead4,
	0xf472b6,
	0x60a5fa,
	0xfacc15,
	0xfb7185,
	0x34d399,
}

type fleetItem struct {
	id     int
	fleet  *Fleet
	target *Planet
}

type Engine struct {
	mu              sync.RWMutex
	tick            int64
	tickRate        int
	fleetSpeed      float64
	growthTimer     float64
	worldWidth      float64
	worldHeight     float64
	planets         map[int]*Planet
	planetIndex     *planetSpatialIndex
	sortedPlanetIDs []int
	fleets          map[int]*Fleet
	sortedFleetIDs  []int
	nextFleetID     int
	mapName         string
	playerColors    []PlayerColor
	// tickBuf holds slices reused across every Advance() call to amortize
	// per-tick allocations. Only accessed while mu write-lock is held.
	tickBuf struct {
		fleetIDs       []int
		items          []fleetItem
		arrived        []bool
		arrivals       []arrivalEvent
		steeringIndex  fleetSpatialIndex
		collisionIndex fleetSpatialIndex
	}
}

const (
	DefaultTickRate      = 20
	defaultFleetSpeedUPS = 120.0
)

func NewEngine() *Engine {
	return NewEngineWithConfig(DefaultMapConfig())
}

func NewEngineWithConfig(config MapConfig) *Engine {
	config = normalizeMapConfig(config)
	mapLayout := newRandomMapLayoutWithConfig(config)

	sortedPlanetIDs := make([]int, 0, len(mapLayout.Planets))
	for id := range mapLayout.Planets {
		sortedPlanetIDs = append(sortedPlanetIDs, id)
	}
	slices.Sort(sortedPlanetIDs)

	return &Engine{
		tickRate:        DefaultTickRate,
		fleetSpeed:      defaultFleetSpeedUPS,
		worldWidth:      mapLayout.Width,
		worldHeight:     mapLayout.Height,
		planets:         mapLayout.Planets,
		planetIndex:     newPlanetSpatialIndex(mapLayout.Planets),
		sortedPlanetIDs: sortedPlanetIDs,
		fleets:          make(map[int]*Fleet),
		sortedFleetIDs:  make([]int, 0),
		nextFleetID:     1,
		mapName:         mapLayout.Name,
		playerColors:    resolvePlayerColors(config.PlayerCount),
	}
}

func resolvePlayerColors(playerCount int) []PlayerColor {
	colors := make([]PlayerColor, 0, playerCount)
	for playerIndex := range playerCount {
		colors = append(colors, PlayerColor{
			PlayerID: playerIndex + 1,
			Color:    playerColorPalette[playerIndex%len(playerColorPalette)],
		})
	}

	return colors
}

func (engine *Engine) TickRate() int {
	return engine.tickRate
}

func (engine *Engine) MapName() string {
	return engine.mapName
}

func (engine *Engine) Tick() int64 {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	return engine.tick
}

func (engine *Engine) Advance(deltaSeconds float64) (int, bool) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	engine.tick++
	engine.moveFleets(deltaSeconds)
	engine.growPlanets(deltaSeconds)

	return engine.checkWinner()
}

func (engine *Engine) Snapshot() Snapshot {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	return engine.buildSnapshot()
}

func (engine *Engine) SnapshotForPlayer(playerID int) Snapshot {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	return engine.buildSnapshotForPlayer(playerID)
}

func (engine *Engine) Winner() (int, bool) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	return engine.checkWinner()
}

func (engine *Engine) checkWinner() (int, bool) {
	winnerID := 0
	for _, planet := range engine.planets {
		if planet.Owner == 0 {
			continue
		}
		if winnerID == 0 {
			winnerID = planet.Owner
			continue
		}
		if planet.Owner != winnerID {
			return 0, false
		}
	}

	for _, fleet := range engine.fleets {
		if fleet.Owner == 0 {
			continue
		}
		if winnerID == 0 {
			winnerID = fleet.Owner
			continue
		}
		if fleet.Owner != winnerID {
			return 0, false
		}
	}

	if winnerID == 0 {
		return 0, false
	}

	return winnerID, true
}

func (engine *Engine) SendFleet(playerID, sourceID, targetID, percentage int) (Fleet, error) {
	if percentage < 1 || percentage > 100 {
		return Fleet{}, ErrInvalidPercentage
	}

	if sourceID == targetID {
		return Fleet{}, ErrSamePlanet
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()

	source, ok := engine.planets[sourceID]
	if !ok {
		return Fleet{}, ErrUnknownPlanet
	}

	target, ok := engine.planets[targetID]
	if !ok {
		return Fleet{}, ErrUnknownPlanet
	}

	if source.Owner != playerID {
		return Fleet{}, ErrNotOwner
	}

	shipsToSend := source.Ships * percentage / 100
	if shipsToSend < 1 {
		return Fleet{}, ErrNoShips
	}

	source.Ships -= shipsToSend
	if source.Ships < 0 {
		source.Ships += shipsToSend
		return Fleet{}, ErrNoShips
	}

	travelSeconds, vx, vy := engine.travelVector(source, target)
	travelTicks := int64(math.Ceil(travelSeconds * float64(engine.tickRate)))
	travelTicks = max(travelTicks, 1)

	launchDirectionX, launchDirectionY := normalizeVector(target.X-source.X, target.Y-source.Y)
	launchOffset := source.Radius + fleetCollisionRadius + collisionPadding
	if launchDirectionX == 0 && launchDirectionY == 0 {
		launchDirectionX = 1
		launchDirectionY = 0
	}

	baseAngle := math.Atan2(launchDirectionY, launchDirectionX)
	launchBundleSize := launchFleetBundleSize(len(engine.fleets), shipsToSend)
	firstFleet := Fleet{}
	remainingShips := shipsToSend
	shipIndex := 0
	for ringIndex := 0; remainingShips > 0; ringIndex++ {
		ringRadius := launchOffset + float64(ringIndex)*shipLaunchSpacing
		ringCapacity := maxShipsOnLaunchRing(ringRadius)
		bundlesInRing := remainingShips / launchBundleSize
		if remainingShips%launchBundleSize != 0 {
			bundlesInRing++
		}
		if bundlesInRing > ringCapacity {
			bundlesInRing = ringCapacity
		}

		for ringSlot := 0; ringSlot < bundlesInRing && remainingShips > 0; ringSlot++ {
			spawnAngle := baseAngle
			if bundlesInRing > 1 {
				spawnAngle += 2 * math.Pi * float64(ringSlot) / float64(bundlesInRing)
			}

			spawnX := source.X + math.Cos(spawnAngle)*ringRadius
			spawnY := source.Y + math.Sin(spawnAngle)*ringRadius
			bundleShips := remainingShips
			bundleShips = min(bundleShips, launchBundleSize)
			fleet := &Fleet{
				ID:         engine.nextFleetID,
				Owner:      playerID,
				SourceID:   sourceID,
				TargetID:   targetID,
				Ships:      bundleShips,
				LaunchTick: engine.tick,
				ETA:        engine.tick + travelTicks,
				X:          spawnX,
				Y:          spawnY,
				VX:         vx,
				VY:         vy,
			}

			engine.fleets[fleet.ID] = fleet
			// Fleet IDs are monotonically increasing, so appending keeps the slice sorted.
			engine.sortedFleetIDs = append(engine.sortedFleetIDs, fleet.ID)
			if shipIndex == 0 {
				firstFleet = *fleet
			}
			engine.nextFleetID++
			shipIndex++
			remainingShips -= bundleShips
		}
	}

	return firstFleet, nil
}

func launchFleetBundleSize(currentFleetCount, shipsToSend int) int {
	projectedFleetCount := currentFleetCount + shipsToSend
	if projectedFleetCount < fleetMergeActivationStep {
		return 1
	}

	return dynamicFleetMergeMaxShips(projectedFleetCount)
}

func maxShipsOnLaunchRing(radius float64) int {
	if radius <= 0 {
		return 1
	}

	capacity := int(math.Floor((2 * math.Pi * radius) / shipLaunchSpacing))
	if capacity < 1 {
		return 1
	}

	return capacity
}

// parallelFleetThreshold is the minimum fleet count before parallel
// advancement is used. Below this the goroutine overhead exceeds the gain.
const parallelFleetThreshold = 64

type arrivalEvent struct {
	id     int
	fleet  *Fleet
	target *Planet
}

func (engine *Engine) moveFleets(deltaSeconds float64) {
	buf := &engine.tickBuf
	// Reuse the fleet-ID snapshot buffer; avoids a new allocation every tick.
	buf.fleetIDs = append(buf.fleetIDs[:0], engine.sortedFleetIDs...)

	buf.steeringIndex.reset(engine.fleets, fleetSeparationDistance+fleetInfluencePadding)
	planetIndex := engine.planetIndex
	if planetIndex == nil {
		planetIndex = newPlanetSpatialIndex(engine.planets)
		engine.planetIndex = planetIndex
	}

	buf.arrivals = buf.arrivals[:0]
	if len(buf.fleetIDs) >= parallelFleetThreshold {
		engine.moveFleetsParallel(buf.fleetIDs, &buf.steeringIndex, planetIndex, deltaSeconds)
	} else {
		engine.moveFleetsSerial(buf.fleetIDs, &buf.steeringIndex, planetIndex, deltaSeconds)
	}

	engine.applyArrivals(buf.arrivals)

	buf.collisionIndex.reset(engine.fleets, fleetSeparationDistance)
	engine.resolveFleetCollisions(&buf.collisionIndex)
	engine.mergeFleets(&buf.collisionIndex)
}

// applyArrivals resolves the planet side-effects of all arrived fleets and
// removes them from the engine. Arrivals are processed serially: with at most
// ~20 per tick the goroutine-spawn overhead of a concurrent approach exceeds
// any parallel benefit, and avoids map/goroutine allocations entirely.
func (engine *Engine) applyArrivals(arrivals []arrivalEvent) {
	for _, ev := range arrivals {
		ev.fleet.X = ev.target.X
		ev.fleet.Y = ev.target.Y
		if ev.target.Owner == ev.fleet.Owner {
			ev.target.Ships += ev.fleet.Ships
		} else {
			ev.target.Ships -= ev.fleet.Ships
			if ev.target.Ships < 0 {
				ev.target.Owner = ev.fleet.Owner
				ev.target.Ships = -ev.target.Ships
			}
		}
		engine.removeSortedFleetID(ev.id)
		delete(engine.fleets, ev.id)
	}
}

// moveFleetsSerial advances fleets one-by-one on the calling goroutine.
// Arrivals are appended directly to engine.tickBuf.arrivals.
func (engine *Engine) moveFleetsSerial(fleetIDs []int, steeringIndex *fleetSpatialIndex, planetIndex *planetSpatialIndex, deltaSeconds float64) {
	for _, id := range fleetIDs {
		fleet := engine.fleets[id]
		if fleet == nil {
			continue
		}
		target := engine.planets[fleet.TargetID]
		if target == nil {
			continue
		}
		if engine.advanceFleet(fleet, target, steeringIndex, planetIndex, deltaSeconds) {
			engine.tickBuf.arrivals = append(engine.tickBuf.arrivals, arrivalEvent{id: id, fleet: fleet, target: target})
		}
	}
}

// moveFleetsParallel advances fleets concurrently across runtime.NumCPU()*4
// workers. Each worker owns a disjoint chunk of fleet items so no
// synchronization is needed during advancement. Arrivals are appended
// directly to engine.tickBuf.arrivals after all workers finish.
// All intermediate slices (items, arrived) are reused from engine.tickBuf.
func (engine *Engine) moveFleetsParallel(fleetIDs []int, steeringIndex *fleetSpatialIndex, planetIndex *planetSpatialIndex, deltaSeconds float64) {
	buf := &engine.tickBuf

	buf.items = buf.items[:0]
	for _, id := range fleetIDs {
		fleet := engine.fleets[id]
		if fleet == nil {
			continue
		}
		target := engine.planets[fleet.TargetID]
		if target == nil {
			continue
		}
		buf.items = append(buf.items, fleetItem{id: id, fleet: fleet, target: target})
	}

	n := len(buf.items)
	if n == 0 {
		return
	}

	// Grow or reuse the arrived flag slice; clear any stale true values.
	if cap(buf.arrived) < n {
		buf.arrived = make([]bool, n)
	} else {
		buf.arrived = buf.arrived[:n]
		clear(buf.arrived)
	}

	numWorkers := min(runtime.NumCPU()*4, n)
	chunkSize := (n + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	for w := range numWorkers {
		start := w * chunkSize
		if start >= n {
			break
		}
		end := min(start+chunkSize, n)
		wg.Add(1)
		go func(slice []fleetItem, out []bool) {
			defer wg.Done()
			for i, it := range slice {
				out[i] = engine.advanceFleet(it.fleet, it.target, steeringIndex, planetIndex, deltaSeconds)
			}
		}(buf.items[start:end], buf.arrived[start:end])
	}
	wg.Wait()

	for i, it := range buf.items {
		if buf.arrived[i] {
			buf.arrivals = append(buf.arrivals, arrivalEvent{id: it.id, fleet: it.fleet, target: it.target})
		}
	}
}

func (engine *Engine) growPlanets(deltaSeconds float64) {
	if deltaSeconds <= 0 {
		return
	}

	engine.growthTimer += deltaSeconds
	if engine.growthTimer+growthTimeEpsilon < planetGrowthIntervalSeconds {
		return
	}

	growthSteps := int((engine.growthTimer + growthTimeEpsilon) / planetGrowthIntervalSeconds)
	engine.growthTimer -= float64(growthSteps) * planetGrowthIntervalSeconds

	for _, planet := range engine.planets {
		if planet.Owner == 0 {
			continue
		}

		planet.Ships += planet.Growth * growthSteps
	}
}

func (engine *Engine) travelVector(source, target *Planet) (float64, float64, float64) {
	dx := target.X - source.X
	dy := target.Y - source.Y
	distance := math.Hypot(dx, dy)
	travelSeconds := distance / engine.fleetSpeed
	if travelSeconds == 0 {
		return 0, 0, 0
	}

	return travelSeconds, dx / travelSeconds, dy / travelSeconds
}

func (engine *Engine) buildSnapshot() Snapshot {
	return engine.buildSnapshotForPlayer(0)
}

func (engine *Engine) buildSnapshotForPlayer(playerID int) Snapshot {
	planetIDs := engine.sortedPlanetIDs
	if len(planetIDs) != len(engine.planets) {
		// Fallback for engines not created via NewEngineWithConfig (e.g. tests).
		planetIDs = make([]int, 0, len(engine.planets))
		for id := range engine.planets {
			planetIDs = append(planetIDs, id)
		}
		slices.Sort(planetIDs)
	}

	planets := make([]Planet, 0, len(planetIDs))
	for _, id := range planetIDs {
		planet := *engine.planets[id]
		if playerID != 0 {
			planet.Growth = 0
			if planet.Owner != 0 && planet.Owner != playerID {
				planet.Ships = 0
			}
		}
		planets = append(planets, planet)
	}

	fleets := make([]Fleet, 0, len(engine.sortedFleetIDs))
	for _, id := range engine.sortedFleetIDs {
		fleet := engine.fleets[id]
		if fleet != nil {
			fleets = append(fleets, *fleet)
		}
	}

	return Snapshot{
		Tick:         engine.tick,
		TickRate:     engine.tickRate,
		Width:        engine.worldWidth,
		Height:       engine.worldHeight,
		Planets:      planets,
		Fleets:       fleets,
		PlayerColors: append([]PlayerColor(nil), engine.playerColors...),
	}
}

// removeSortedFleetID removes id from the sorted fleet ID slice in O(log N)
// search + O(N) shift. Called on every fleet deletion (arrival, merge).
func (engine *Engine) removeSortedFleetID(id int) {
	idx, found := slices.BinarySearch(engine.sortedFleetIDs, id)
	if !found {
		return
	}
	engine.sortedFleetIDs = append(engine.sortedFleetIDs[:idx], engine.sortedFleetIDs[idx+1:]...)
}
