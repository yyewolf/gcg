package game

import (
	"math"
	"sort"
	"sync"
)

const shipLaunchSpacing = 1.5

const launchPositionTolerance = 0.001

type Engine struct {
	mu          sync.RWMutex
	tick        int64
	tickRate    int
	fleetSpeed  float64
	planets     map[int]*Planet
	fleets      map[int]*Fleet
	nextFleetID int
	mapName     string
}

func NewEngine() *Engine {
	return &Engine{
		tickRate:    DefaultTickRate,
		fleetSpeed:  defaultFleetSpeedUPS,
		planets:     starterPlanets(),
		fleets:      make(map[int]*Fleet),
		nextFleetID: 1,
		mapName:     "starter",
	}
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
	engine.moveFleetsLocked()
	engine.growPlanetsLocked()

	return engine.snapshotLocked()
}

func (engine *Engine) Snapshot() Snapshot {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	return engine.snapshotLocked()
}

func (engine *Engine) SnapshotForPlayer(playerID int) Snapshot {
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	return engine.snapshotForPlayerLocked(playerID)
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
	if travelTicks < 1 {
		travelTicks = 1
	}

	launchDirectionX, launchDirectionY := normalizeVector(target.X-source.X, target.Y-source.Y)
	launchOffset := source.Radius + fleetCollisionRadius + collisionPadding
	if launchDirectionX == 0 && launchDirectionY == 0 {
		launchDirectionX = 1
		launchDirectionY = 0
	}

	baseAngle := math.Atan2(launchDirectionY, launchDirectionX)
	firstFleet := Fleet{}
	remainingShips := shipsToSend
	shipIndex := 0
	for ringIndex := 0; remainingShips > 0; ringIndex++ {
		ringRadius := launchOffset + float64(ringIndex)*shipLaunchSpacing
		ringCapacity := maxShipsOnLaunchRing(ringRadius)
		shipsInRing := remainingShips
		if shipsInRing > ringCapacity {
			shipsInRing = ringCapacity
		}

		for ringSlot := 0; ringSlot < shipsInRing; ringSlot++ {
			spawnAngle := baseAngle
			if shipsInRing > 1 {
				spawnAngle += 2 * math.Pi * float64(ringSlot) / float64(shipsInRing)
			}

			spawnX := source.X + math.Cos(spawnAngle)*ringRadius
			spawnY := source.Y + math.Sin(spawnAngle)*ringRadius

			fleet := &Fleet{
				ID:         engine.nextFleetID,
				Owner:      playerID,
				SourceID:   sourceID,
				TargetID:   targetID,
				Ships:      1,
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
		}

		remainingShips -= shipsInRing
	}

	return firstFleet, nil
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

func (engine *Engine) moveFleetsLocked() {
	fleetIDs := make([]int, 0, len(engine.fleets))
	for id := range engine.fleets {
		fleetIDs = append(fleetIDs, id)
	}
	sort.Ints(fleetIDs)
	steeringIndex := newFleetSpatialIndex(engine.fleets, fleetSeparationDistance+fleetInfluencePadding)

	for _, id := range fleetIDs {
		fleet := engine.fleets[id]
		if fleet == nil {
			continue
		}

		target := engine.planets[fleet.TargetID]
		if target == nil {
			continue
		}

		engine.advanceFleetLocked(id, fleet, target, steeringIndex)
	}

	collisionIndex := newFleetSpatialIndex(engine.fleets, fleetSeparationDistance)
	engine.resolveFleetCollisionsLocked(collisionIndex)

	for _, id := range fleetIDs {
		fleet := engine.fleets[id]
		if fleet == nil {
			continue
		}

		target := engine.planets[fleet.TargetID]
		if target == nil {
			continue
		}

		fleet.ETA = engine.tick + engine.estimateRemainingTicksLocked(fleet, target)
	}
}

func (engine *Engine) growPlanetsLocked() {
	if engine.tick%int64(engine.tickRate) != 0 {
		return
	}

	for _, planet := range engine.planets {
		if planet.Owner == 0 {
			continue
		}

		planet.Ships += planet.Growth
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

func (engine *Engine) estimateRemainingTicksLocked(fleet *Fleet, target *Planet) int64 {
	distance := math.Hypot(target.X-fleet.X, target.Y-fleet.Y) - (target.Radius + fleetCollisionRadius)
	if distance <= 0 {
		return 0
	}

	travelSeconds := distance / engine.fleetSpeed
	ticks := int64(math.Ceil(travelSeconds * float64(engine.tickRate)))
	if ticks < 1 {
		return 1
	}

	return ticks
}

func (engine *Engine) snapshotLocked() Snapshot {
	return engine.snapshotForPlayerLocked(0)
}

func (engine *Engine) snapshotForPlayerLocked(playerID int) Snapshot {
	planetIDs := make([]int, 0, len(engine.planets))
	for id := range engine.planets {
		planetIDs = append(planetIDs, id)
	}
	sort.Ints(planetIDs)

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
	sort.Ints(fleetIDs)

	fleets := make([]Fleet, 0, len(fleetIDs))
	for _, id := range fleetIDs {
		fleets = append(fleets, *engine.fleets[id])
	}

	return Snapshot{
		Tick:     engine.tick,
		TickRate: engine.tickRate,
		Planets:  planets,
		Fleets:   fleets,
	}
}
