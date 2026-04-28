package game

import (
	"math"
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

type Engine struct {
	mu           sync.RWMutex
	tick         int64
	tickRate     int
	fleetSpeed   float64
	growthTimer  float64
	worldWidth   float64
	worldHeight  float64
	planets      map[int]*Planet
	planetIndex  *planetSpatialIndex
	fleets       map[int]*Fleet
	nextFleetID  int
	mapName      string
	playerColors []PlayerColor
}

const (
	DefaultTickRate      = 15
	DefaultIdleTickRate  = 5
	defaultFleetSpeedUPS = 110.0
)

func NewEngine() *Engine {
	return NewEngineWithConfig(DefaultMapConfig())
}

func NewEngineWithConfig(config MapConfig) *Engine {
	config = normalizeMapConfig(config)
	mapLayout := newRandomMapLayoutWithConfig(config)

	return &Engine{
		tickRate:     DefaultIdleTickRate,
		fleetSpeed:   defaultFleetSpeedUPS,
		worldWidth:   mapLayout.Width,
		worldHeight:  mapLayout.Height,
		planets:      mapLayout.Planets,
		planetIndex:  newPlanetSpatialIndex(mapLayout.Planets),
		fleets:       make(map[int]*Fleet),
		nextFleetID:  1,
		mapName:      mapLayout.Name,
		playerColors: resolvePlayerColors(config.PlayerCount),
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

func (engine *Engine) Advance() Snapshot {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	engine.tick++
	engine.moveFleets()
	engine.growPlanets()
	engine.tickRate = engine.resolveDynamicTickRate()

	return engine.buildSnapshot()
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
			if shipIndex == 0 {
				firstFleet = *fleet
			}
			engine.nextFleetID++
			shipIndex++
			remainingShips -= bundleShips
		}
	}

	engine.tickRate = engine.resolveDynamicTickRate()

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

func (engine *Engine) moveFleets() {
	fleetIDs := make([]int, 0, len(engine.fleets))
	for id := range engine.fleets {
		fleetIDs = append(fleetIDs, id)
	}
	slices.Sort(fleetIDs)
	steeringIndex := newFleetSpatialIndex(engine.fleets, fleetSeparationDistance+fleetInfluencePadding)
	planetIndex := engine.planetIndex
	if planetIndex == nil {
		planetIndex = newPlanetSpatialIndex(engine.planets)
		engine.planetIndex = planetIndex
	}

	for _, id := range fleetIDs {
		fleet := engine.fleets[id]
		if fleet == nil {
			continue
		}

		target := engine.planets[fleet.TargetID]
		if target == nil {
			continue
		}

		engine.advanceFleet(id, fleet, target, steeringIndex, planetIndex)
	}

	collisionIndex := newFleetSpatialIndex(engine.fleets, fleetSeparationDistance)
	engine.resolveFleetCollisions(collisionIndex)
	engine.mergeFleets(collisionIndex)
}

func (engine *Engine) growPlanets() {
	if engine.tickRate < 1 {
		return
	}

	engine.growthTimer += 1 / float64(engine.tickRate)
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
	planetIDs := make([]int, 0, len(engine.planets))
	for id := range engine.planets {
		planetIDs = append(planetIDs, id)
	}
	slices.Sort(planetIDs)

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

	fleetIDs := make([]int, 0, len(engine.fleets))
	for id := range engine.fleets {
		fleetIDs = append(fleetIDs, id)
	}
	slices.Sort(fleetIDs)

	fleets := make([]Fleet, 0, len(fleetIDs))
	for _, id := range fleetIDs {
		fleets = append(fleets, *engine.fleets[id])
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
