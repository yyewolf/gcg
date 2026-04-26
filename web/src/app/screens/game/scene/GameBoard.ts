import { Container, Graphics } from "pixi.js";

import type {
  FleetSnapshot,
  PlanetSnapshot,
  Snapshot,
} from "../../../game/protocol";

import { worldLayout, palette, type OwnershipTone } from "../theme";

import { FleetView } from "./FleetView";

import { PlanetView } from "./PlanetView";

interface Star {
  x: number;
  y: number;
  radius: number;
  alpha: number;
}

const showDebugFleetTrails = false;

const STAR_FIELD: Star[] = Array.from({ length: 48 }, (_, index) => ({
  x: (index * 137) % worldLayout.worldWidth,
  y: (index * 79) % worldLayout.worldHeight,
  radius: (index % 3) + 1,
  alpha: 0.16 + (index % 5) * 0.08,
}));

export class GameBoard extends Container {
  private readonly screenBackdrop = new Graphics();
  private readonly world = new Container();
  private readonly worldBackdrop = new Graphics();
  private readonly fleetLayer = new Container();
  private readonly planetLayer = new Container();
  private readonly fleetViews = new Map<number, FleetView>();
  private readonly planetViews = new Map<number, PlanetView>();
  private readonly previousFleets = new Map<number, FleetSnapshot>();
  private readonly previousPlanets = new Map<number, PlanetSnapshot>();

  constructor(private readonly onPlanetTap: (planetID: number) => void) {
    super();

    this.world.addChild(this.worldBackdrop, this.fleetLayer, this.planetLayer);
    this.addChild(this.screenBackdrop, this.world);
    this.drawWorldBackdrop();
  }

  public resize(width: number, height: number): void {
    const availableHeight = Math.max(height - worldLayout.hudHeight, 260);
    const scale = Math.min(
      (width - worldLayout.padding * 2) / worldLayout.worldWidth,
      (availableHeight - worldLayout.padding * 2) / worldLayout.worldHeight,
    );

    const scaledWidth = worldLayout.worldWidth * scale;
    const scaledHeight = worldLayout.worldHeight * scale;
    const worldX = (width - scaledWidth) * 0.5;
    const worldY =
      worldLayout.hudHeight + (availableHeight - scaledHeight) * 0.5;

    this.screenBackdrop.clear();
    this.screenBackdrop.rect(0, 0, width, height);
    this.screenBackdrop.fill({ color: palette.background, alpha: 1 });
    this.screenBackdrop.rect(
      0,
      worldLayout.hudHeight,
      width,
      height - worldLayout.hudHeight,
    );
    this.screenBackdrop.fill({ color: palette.surface, alpha: 0.9 });

    this.world.position.set(worldX, worldY);
    this.world.scale.set(scale);
  }

  public sync(
    snapshot: Snapshot | null,
    playerID: number | null,
    selectedSourceID: number | null,
  ): void {
    if (snapshot === null) {
      this.syncPlanets([], playerID, selectedSourceID);
      this.syncFleets([], playerID);
      this.previousFleets.clear();
      this.previousPlanets.clear();
      return;
    }

    this.syncPlanets(snapshot.planets, playerID, selectedSourceID);
    this.syncLandingImpacts(snapshot.fleets, playerID);
    this.syncFleets(snapshot.fleets, playerID);
    this.previousPlanets.clear();
    for (const planet of snapshot.planets) {
      this.previousPlanets.set(planet.id, planet);
    }
  }

  public update(deltaMS: number): void {
    for (const planetView of this.planetViews.values()) {
      planetView.update(deltaMS);
    }
  }

  private drawWorldBackdrop(): void {
    this.worldBackdrop.clear();
    this.worldBackdrop.roundRect(
      0,
      0,
      worldLayout.worldWidth,
      worldLayout.worldHeight,
      34,
    );
    this.worldBackdrop.fill({ color: palette.panel, alpha: 1 });
    this.worldBackdrop.roundRect(
      0,
      0,
      worldLayout.worldWidth,
      worldLayout.worldHeight,
      34,
    );
    this.worldBackdrop.stroke({ color: palette.outline, width: 3, alpha: 1 });

    for (const star of STAR_FIELD) {
      this.worldBackdrop.circle(star.x, star.y, star.radius);
      this.worldBackdrop.fill({ color: 0xffffff, alpha: star.alpha });
    }

    this.worldBackdrop.circle(170, 120, 150);
    this.worldBackdrop.fill({ color: palette.friendly, alpha: 0.06 });
    this.worldBackdrop.circle(620, 280, 180);
    this.worldBackdrop.fill({ color: palette.enemy, alpha: 0.05 });

    for (let x = 80; x < worldLayout.worldWidth; x += 80) {
      this.worldBackdrop.moveTo(x, 32);
      this.worldBackdrop.lineTo(x, worldLayout.worldHeight - 32);
    }
    for (let y = 64; y < worldLayout.worldHeight; y += 64) {
      this.worldBackdrop.moveTo(32, y);
      this.worldBackdrop.lineTo(worldLayout.worldWidth - 32, y);
    }
    this.worldBackdrop.stroke({
      color: palette.outline,
      width: 1,
      alpha: 0.18,
    });
  }

  private syncPlanets(
    planets: PlanetSnapshot[],
    playerID: number | null,
    selectedSourceID: number | null,
  ): void {
    const activeIDs = new Set<number>();

    for (const planet of planets) {
      activeIDs.add(planet.id);
      let view = this.planetViews.get(planet.id);
      if (view === undefined) {
        view = new PlanetView(this.onPlanetTap);
        this.planetViews.set(planet.id, view);
        this.planetLayer.addChild(view);
      }

      view.sync({
        planet,
        selected: planet.id === selectedSourceID,
        tone: this.resolveTone(planet.owner, playerID),
      });
    }

    for (const [planetID, view] of this.planetViews) {
      if (activeIDs.has(planetID)) {
        continue;
      }

      this.planetViews.delete(planetID);
      view.destroy();
    }
  }

  private syncFleets(fleets: FleetSnapshot[], playerID: number | null): void {
    const activeIDs = new Set<number>();

    for (const fleet of fleets) {
      activeIDs.add(fleet.id);
      let view = this.fleetViews.get(fleet.id);
      if (view === undefined) {
        view = new FleetView(showDebugFleetTrails);
        this.fleetViews.set(fleet.id, view);
        this.fleetLayer.addChild(view);
      }

      view.sync(fleet, fleet.owner === playerID);
    }

    for (const [fleetID, view] of this.fleetViews) {
      if (activeIDs.has(fleetID)) {
        continue;
      }

      this.fleetViews.delete(fleetID);
      view.destroy();
    }
  }

  private syncLandingImpacts(
    fleets: FleetSnapshot[],
    playerID: number | null,
  ): void {
    const activeIDs = new Set<number>();
    for (const fleet of fleets) {
      activeIDs.add(fleet.id);
    }

    for (const [fleetID, fleet] of this.previousFleets) {
      if (activeIDs.has(fleetID)) {
        continue;
      }

      const targetView = this.planetViews.get(fleet.dst);
      const previousTarget = this.previousPlanets.get(fleet.dst);
      if (
        targetView !== undefined &&
        previousTarget !== undefined &&
        (playerID === null || previousTarget.owner !== playerID)
      ) {
        targetView.addImpact();
      }
    }

    this.previousFleets.clear();
    for (const fleet of fleets) {
      this.previousFleets.set(fleet.id, fleet);
    }
  }

  private resolveTone(owner: number, playerID: number | null): OwnershipTone {
    if (owner === 0) {
      return "neutral";
    }

    if (playerID !== null && owner === playerID) {
      return "self";
    }

    return "enemy";
  }
}
