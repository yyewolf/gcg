package game

import "testing"

func TestRandomMapLayoutProducesLargeDenseTenPlayerMap(t *testing.T) {
	t.Parallel()

	layout := newRandomMapLayoutWithConfig(MapConfig{PlayerCount: 10})

	if layout.Width < 3000 {
		t.Fatalf("expected wider map, got width %.0f", layout.Width)
	}
	if layout.Height < 2000 {
		t.Fatalf("expected taller map, got height %.0f", layout.Height)
	}
	if len(layout.Planets) < 55 {
		t.Fatalf("expected a dense planet field, got %d planets", len(layout.Planets))
	}

	homeCount := 0
	owners := make(map[int]bool)
	for _, planet := range layout.Planets {
		if planet.Owner != 0 {
			homeCount++
			owners[planet.Owner] = true
		}
	}

	if homeCount != 10 {
		t.Fatalf("expected %d owned home planets, got %d", 10, homeCount)
	}
	for owner := 1; owner <= 10; owner++ {
		if !owners[owner] {
			t.Fatalf("missing home planet for player %d", owner)
		}
	}

	for _, planet := range layout.Planets {
		if planet.Owner == 0 {
			continue
		}
		if planet.Ships != homePlanetShips {
			t.Fatalf("expected home planet %d to start with %d ships, got %d", planet.ID, homePlanetShips, planet.Ships)
		}
	}

	for _, planet := range layout.Planets {
		if planet.X-planet.Radius < mapEdgePadding || planet.X+planet.Radius > layout.Width-mapEdgePadding {
			t.Fatalf("planet %d exceeds horizontal bounds", planet.ID)
		}
		if planet.Y-planet.Radius < mapEdgePadding || planet.Y+planet.Radius > layout.Height-mapEdgePadding {
			t.Fatalf("planet %d exceeds vertical bounds", planet.ID)
		}
	}
}

func TestRandomMapLayoutHonorsConfiguredPlayerCount(t *testing.T) {
	t.Parallel()

	layout := newRandomMapLayoutWithConfig(MapConfig{PlayerCount: 6})

	owners := make(map[int]bool)
	for _, planet := range layout.Planets {
		if planet.Owner != 0 {
			owners[planet.Owner] = true
		}
	}

	if len(owners) != 6 {
		t.Fatalf("expected 6 owned home planets, got %d", len(owners))
	}
	for owner := 1; owner <= 6; owner++ {
		if !owners[owner] {
			t.Fatalf("missing home planet for player %d", owner)
		}
	}

	for _, planet := range layout.Planets {
		if planet.Owner == 0 {
			continue
		}
		if planet.Ships != homePlanetShips {
			t.Fatalf("expected player %d home planet to start with %d ships, got %d", planet.Owner, homePlanetShips, planet.Ships)
		}
	}
}

func TestRandomMapLayoutScalesForMorePlayers(t *testing.T) {
	t.Parallel()

	sixPlayerLayout := newRandomMapLayoutWithConfig(MapConfig{PlayerCount: 6})
	twelvePlayerLayout := newRandomMapLayoutWithConfig(MapConfig{PlayerCount: 12})

	if twelvePlayerLayout.Width <= sixPlayerLayout.Width {
		t.Fatalf("expected 12-player map to be wider than 6-player map, got %.0f <= %.0f", twelvePlayerLayout.Width, sixPlayerLayout.Width)
	}
	if twelvePlayerLayout.Height <= sixPlayerLayout.Height {
		t.Fatalf("expected 12-player map to be taller than 6-player map, got %.0f <= %.0f", twelvePlayerLayout.Height, sixPlayerLayout.Height)
	}
}
