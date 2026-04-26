package game

import "math"

const (
	fleetCollisionRadius = 6.0
	collisionPadding     = 4.0
	avoidancePadding     = 8.0
)

func distanceSquared(ax, ay, bx, by float64) float64 {
	dx := ax - bx
	dy := ay - by
	return dx*dx + dy*dy
}

func normalizeVector(x, y float64) (float64, float64) {
	length := math.Hypot(x, y)
	if length == 0 {
		return 0, 0
	}

	return x / length, y / length
}

func segmentIntersectsCircle(startX, startY, endX, endY float64, planet *Planet, padding float64) bool {
	radius := planet.Radius + padding
	segmentX := endX - startX
	segmentY := endY - startY
	segmentLengthSquared := segmentX*segmentX + segmentY*segmentY
	if segmentLengthSquared == 0 {
		return distanceSquared(startX, startY, planet.X, planet.Y) <= radius*radius
	}

	projection := ((planet.X-startX)*segmentX + (planet.Y-startY)*segmentY) / segmentLengthSquared
	if projection < 0 {
		projection = 0
	} else if projection > 1 {
		projection = 1
	}

	closestX := startX + projection*segmentX
	closestY := startY + projection*segmentY
	return distanceSquared(closestX, closestY, planet.X, planet.Y) <= radius*radius
}

func tangentVector(normalX, normalY float64, clockwise bool) (float64, float64) {
	if clockwise {
		return normalY, -normalX
	}

	return -normalY, normalX
}
