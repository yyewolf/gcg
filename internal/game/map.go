package game

func starterPlanets() map[int]*Planet {
	return map[int]*Planet{
		1: {ID: 1, X: 140, Y: 180, Radius: 34, Owner: 1, Ships: 60, Growth: 4},
		2: {ID: 2, X: 660, Y: 180, Radius: 34, Owner: 2, Ships: 60, Growth: 4},
		3: {ID: 3, X: 400, Y: 90, Radius: 24, Owner: 0, Ships: 28, Growth: 2},
		4: {ID: 4, X: 400, Y: 270, Radius: 24, Owner: 0, Ships: 28, Growth: 2},
		5: {ID: 5, X: 280, Y: 330, Radius: 28, Owner: 0, Ships: 36, Growth: 3},
		6: {ID: 6, X: 520, Y: 330, Radius: 28, Owner: 0, Ships: 36, Growth: 3},
	}
}
