package game

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	defaultMapWidth          = 3200.0
	defaultMapHeight         = 2200.0
	mapEdgePadding           = 120.0
	defaultPlayerCount       = 10
	minPlayerCount           = 2
	maxPlayerCount           = 12
	homePlanetRadius         = 40.0
	homePlanetShips          = 70
	homePlanetGrowth         = 4
	homeOrbitScaleX          = 0.38
	homeOrbitScaleY          = 0.35
	localNeutralCountPerHome = 2
	centerClusterCount       = 18
	roamingNeutralCount      = 36
	neutralPlanetMinRadius   = 18.0
	neutralPlanetMaxRadius   = 36.0
	planetSpacingPadding     = 26.0
	mapGenerationAttempts    = 512
)

type mapLayout struct {
	Name    string
	Width   float64
	Height  float64
	Planets map[int]*Planet
}

type MapConfig struct {
	PlayerCount int
}

func DefaultMapConfig() MapConfig {
	return MapConfig{PlayerCount: defaultPlayerCount}
}

func normalizeMapConfig(config MapConfig) MapConfig {
	if config.PlayerCount < minPlayerCount {
		config.PlayerCount = minPlayerCount
	}
	if config.PlayerCount > maxPlayerCount {
		config.PlayerCount = maxPlayerCount
	}

	return config
}

func newRandomMapLayoutWithConfig(config MapConfig) mapLayout {
	config = normalizeMapConfig(config)
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	width := defaultMapWidth
	height := defaultMapHeight
	planets := make(map[int]*Planet)
	nextID := addHomePlanets(planets, width, height, config.PlayerCount)
	nextID = addLocalNeutralPlanets(planets, nextID, width, height, config.PlayerCount, rng)
	nextID = addCenterClusterPlanets(planets, nextID, width, height, rng)
	addRoamingNeutralPlanets(planets, nextID, width, height, rng)

	return mapLayout{
		Name:    fmt.Sprintf("random-%dp-%d", config.PlayerCount, seed%100000),
		Width:   width,
		Height:  height,
		Planets: planets,
	}
}

func addHomePlanets(planets map[int]*Planet, width, height float64, playerCount int) int {
	centerX := width * 0.5
	centerY := height * 0.5
	orbitX := width * homeOrbitScaleX
	orbitY := height * homeOrbitScaleY
	startAngle := -math.Pi * 0.5
	nextID := 1

	for playerIndex := 0; playerIndex < playerCount; playerIndex++ {
		angle := startAngle + 2*math.Pi*float64(playerIndex)/float64(playerCount)
		planets[nextID] = &Planet{
			ID:     nextID,
			X:      centerX + math.Cos(angle)*orbitX,
			Y:      centerY + math.Sin(angle)*orbitY,
			Radius: homePlanetRadius,
			Owner:  playerIndex + 1,
			Ships:  homePlanetShips,
			Growth: homePlanetGrowth,
		}
		nextID++
	}

	return nextID
}

func addLocalNeutralPlanets(planets map[int]*Planet, nextID int, width, height float64, playerCount int, rng *rand.Rand) int {
	centerX := width * 0.5
	centerY := height * 0.5

	for homeID := 1; homeID <= playerCount; homeID++ {
		home := planets[homeID]
		if home == nil {
			continue
		}

		inwardX, inwardY := normalizeVector(centerX-home.X, centerY-home.Y)
		tangentX, tangentY := -inwardY, inwardX

		for laneIndex := 0; laneIndex < localNeutralCountPerHome; laneIndex++ {
			placed := false
			for attempt := 0; attempt < mapGenerationAttempts; attempt++ {
				radius := randomNeutralRadius(rng)
				distance := 250 + float64(laneIndex)*170 + randomRange(rng, -45, 45)
				tangentOffset := randomRange(rng, -85, 85)
				x := home.X + inwardX*distance + tangentX*tangentOffset
				y := home.Y + inwardY*distance + tangentY*tangentOffset
				if !positionAvailable(planets, x, y, radius, width, height) {
					continue
				}

				ships, growth := neutralStats(radius)
				planets[nextID] = &Planet{ID: nextID, X: x, Y: y, Radius: radius, Ships: ships, Growth: growth}
				nextID++
				placed = true
				break
			}

			if !placed {
				break
			}
		}
	}

	return nextID
}

func addCenterClusterPlanets(planets map[int]*Planet, nextID int, width, height float64, rng *rand.Rand) int {
	centerX := width * 0.5
	centerY := height * 0.5
	centerRadiusX := width * 0.18
	centerRadiusY := height * 0.18

	for clusterIndex := 0; clusterIndex < centerClusterCount; clusterIndex++ {
		for attempt := 0; attempt < mapGenerationAttempts; attempt++ {
			radius := randomNeutralRadius(rng)
			angle := randomRange(rng, 0, 2*math.Pi)
			distanceScale := math.Sqrt(rng.Float64())
			x := centerX + math.Cos(angle)*centerRadiusX*distanceScale
			y := centerY + math.Sin(angle)*centerRadiusY*distanceScale
			if !positionAvailable(planets, x, y, radius, width, height) {
				continue
			}

			ships, growth := neutralStats(radius)
			planets[nextID] = &Planet{ID: nextID, X: x, Y: y, Radius: radius, Ships: ships, Growth: growth}
			nextID++
			break
		}
	}

	return nextID
}

func addRoamingNeutralPlanets(planets map[int]*Planet, nextID int, width, height float64, rng *rand.Rand) {
	for roamingIndex := 0; roamingIndex < roamingNeutralCount; roamingIndex++ {
		for attempt := 0; attempt < mapGenerationAttempts; attempt++ {
			radius := randomNeutralRadius(rng)
			x := randomRange(rng, mapEdgePadding+radius, width-mapEdgePadding-radius)
			y := randomRange(rng, mapEdgePadding+radius, height-mapEdgePadding-radius)
			if !positionAvailable(planets, x, y, radius, width, height) {
				continue
			}

			ships, growth := neutralStats(radius)
			planets[nextID] = &Planet{ID: nextID, X: x, Y: y, Radius: radius, Ships: ships, Growth: growth}
			nextID++
			break
		}
	}
}

func randomNeutralRadius(rng *rand.Rand) float64 {
	return neutralPlanetMinRadius + rng.Float64()*(neutralPlanetMaxRadius-neutralPlanetMinRadius)
}

func neutralStats(radius float64) (int, int) {
	growth := 1
	if radius >= 27 {
		growth = 2
	}
	if radius >= 32 {
		growth = 3
	}

	ships := int(math.Round(radius * 1.2))
	if ships < 18 {
		ships = 18
	}

	return ships, growth
}

func randomRange(rng *rand.Rand, minValue, maxValue float64) float64 {
	if maxValue <= minValue {
		return minValue
	}

	return minValue + rng.Float64()*(maxValue-minValue)
}

func positionAvailable(planets map[int]*Planet, x, y, radius, width, height float64) bool {
	if x-radius < mapEdgePadding || x+radius > width-mapEdgePadding {
		return false
	}
	if y-radius < mapEdgePadding || y+radius > height-mapEdgePadding {
		return false
	}

	for _, planet := range planets {
		minimumDistance := planet.Radius + radius + planetSpacingPadding
		if distanceSquared(x, y, planet.X, planet.Y) < minimumDistance*minimumDistance {
			return false
		}
	}

	return true
}
