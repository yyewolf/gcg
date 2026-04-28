package game

import "errors"

var (
	ErrUnknownPlanet     = errors.New("planet not found")
	ErrInvalidPercentage = errors.New("percentage must be between 1 and 100")
	ErrSamePlanet        = errors.New("source and destination must be different")
	ErrNotOwner          = errors.New("player does not own source planet")
	ErrNoShips           = errors.New("source planet does not have enough ships")
)
