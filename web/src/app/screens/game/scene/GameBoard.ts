import {
  Container,
  FederatedPointerEvent,
  FederatedWheelEvent,
  Graphics,
  Rectangle,
} from "pixi.js";

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
const minCameraZoom = 1;
const maxCameraZoom = 3.5;
const zoomStep = 1.15;
const panDragThreshold = 8;
const predictionLeadTicks = 1.35;

function buildStarField(width: number, height: number): Star[] {
  const count = Math.max(48, Math.round((width * height) / 28000));

  return Array.from({ length: count }, (_, index) => ({
    x: (((index * 137) % 1000) / 1000) * width,
    y: (((index * 79) % 1000) / 1000) * height,
    radius: (index % 3) + 1,
    alpha: 0.16 + (index % 5) * 0.08,
  }));
}

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
  private mapWidth = worldLayout.worldWidth;
  private mapHeight = worldLayout.worldHeight;
  private viewportWidth = 0;
  private viewportHeight = 0;
  private fitScale = 1;
  private cameraZoom = 1;
  private cameraPanX = 0;
  private cameraPanY = 0;
  private fleetPredictionMS = 0;
  private fleetPredictionLimitMS = 100;
  private pointerDown = false;
  private dragActive = false;
  private dragStartX = 0;
  private dragStartY = 0;
  private dragLastX = 0;
  private dragLastY = 0;

  constructor(private readonly onPlanetTap: (planetID: number) => void) {
    super();

    this.eventMode = "static";
    this.world.addChild(this.worldBackdrop, this.fleetLayer, this.planetLayer);
    this.addChild(this.screenBackdrop, this.world);
    this.on("pointerdown", this.handlePointerDown, this);
    this.on("globalpointermove", this.handlePointerMove, this);
    this.on("pointerup", this.handlePointerUp, this);
    this.on("pointerupoutside", this.handlePointerUp, this);
    this.on("wheel", this.handleWheel, this);
    this.drawWorldBackdrop();
  }

  public resize(width: number, height: number): void {
    this.viewportWidth = width;
    this.viewportHeight = height;
    this.hitArea = new Rectangle(0, 0, width, height);

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

    this.applyWorldTransform();
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
      this.fleetPredictionMS = 0;
      return;
    }

    this.syncWorldBounds(snapshot.width, snapshot.height);
    this.fleetPredictionMS = 0;
    this.fleetPredictionLimitMS = Math.max(
      50,
      (1000 / Math.max(1, snapshot.tickRate)) * predictionLeadTicks,
    );

    this.syncPlanets(snapshot.planets, playerID, selectedSourceID);
    this.syncLandingImpacts(snapshot.fleets, playerID);
    this.syncFleets(snapshot.fleets, playerID);
    this.previousPlanets.clear();
    for (const planet of snapshot.planets) {
      this.previousPlanets.set(planet.id, planet);
    }
  }

  public update(deltaMS: number): void {
    this.fleetPredictionMS = Math.min(
      this.fleetPredictionMS + deltaMS,
      this.fleetPredictionLimitMS,
    );

    for (const planetView of this.planetViews.values()) {
      planetView.update(deltaMS);
    }

    for (const fleetView of this.fleetViews.values()) {
      fleetView.predict(this.fleetPredictionMS);
    }
  }

  private drawWorldBackdrop(): void {
    this.worldBackdrop.clear();
    this.worldBackdrop.roundRect(0, 0, this.mapWidth, this.mapHeight, 34);
    this.worldBackdrop.fill({ color: palette.panel, alpha: 1 });
    this.worldBackdrop.roundRect(0, 0, this.mapWidth, this.mapHeight, 34);
    this.worldBackdrop.stroke({ color: palette.outline, width: 3, alpha: 1 });

    for (const star of buildStarField(this.mapWidth, this.mapHeight)) {
      this.worldBackdrop.circle(star.x, star.y, star.radius);
      this.worldBackdrop.fill({ color: 0xffffff, alpha: star.alpha });
    }

    this.worldBackdrop.circle(this.mapWidth * 0.18, this.mapHeight * 0.28, 150);
    this.worldBackdrop.fill({ color: palette.friendly, alpha: 0.06 });
    this.worldBackdrop.circle(this.mapWidth * 0.76, this.mapHeight * 0.68, 180);
    this.worldBackdrop.fill({ color: palette.enemy, alpha: 0.05 });

    const gridX = Math.max(80, Math.round(this.mapWidth / 14));
    const gridY = Math.max(64, Math.round(this.mapHeight / 10));

    for (let x = gridX; x < this.mapWidth; x += gridX) {
      this.worldBackdrop.moveTo(x, 32);
      this.worldBackdrop.lineTo(x, this.mapHeight - 32);
    }
    for (let y = gridY; y < this.mapHeight; y += gridY) {
      this.worldBackdrop.moveTo(32, y);
      this.worldBackdrop.lineTo(this.mapWidth - 32, y);
    }
    this.worldBackdrop.stroke({
      color: palette.outline,
      width: 1,
      alpha: 0.18,
    });
  }

  private syncWorldBounds(width: number, height: number): void {
    const nextWidth = Math.max(1, width);
    const nextHeight = Math.max(1, height);
    if (nextWidth === this.mapWidth && nextHeight === this.mapHeight) {
      return;
    }

    this.mapWidth = nextWidth;
    this.mapHeight = nextHeight;
    this.drawWorldBackdrop();

    if (this.viewportWidth > 0 && this.viewportHeight > 0) {
      this.resize(this.viewportWidth, this.viewportHeight);
    }
  }

  private handlePointerDown(event: FederatedPointerEvent): void {
    if (event.button !== 0) {
      return;
    }
    if (event.global.y < worldLayout.hudHeight) {
      return;
    }

    this.pointerDown = true;
    this.dragActive = false;
    this.dragStartX = event.global.x;
    this.dragStartY = event.global.y;
    this.dragLastX = event.global.x;
    this.dragLastY = event.global.y;
  }

  private handlePointerMove(event: FederatedPointerEvent): void {
    if (!this.pointerDown) {
      return;
    }

    if (!this.dragActive) {
      const dragDistanceX = event.global.x - this.dragStartX;
      const dragDistanceY = event.global.y - this.dragStartY;
      if (
        dragDistanceX * dragDistanceX + dragDistanceY * dragDistanceY <
        panDragThreshold * panDragThreshold
      ) {
        return;
      }

      this.dragActive = true;
      this.cursor = "grabbing";
    }

    this.cameraPanX += event.global.x - this.dragLastX;
    this.cameraPanY += event.global.y - this.dragLastY;
    this.dragLastX = event.global.x;
    this.dragLastY = event.global.y;
    this.applyWorldTransform();
    event.stopPropagation();
  }

  private handlePointerUp(event: FederatedPointerEvent): void {
    if (!this.pointerDown) {
      return;
    }

    this.pointerDown = false;
    const didDrag = this.dragActive;
    this.dragActive = false;
    this.cursor = "default";
    if (didDrag) {
      event.stopPropagation();
    }
  }

  private handleWheel(event: FederatedWheelEvent): void {
    if (this.viewportWidth < 1 || this.viewportHeight < 1) {
      return;
    }
    if (event.global.y < worldLayout.hudHeight) {
      return;
    }

    const previousScale = this.fitScale * this.cameraZoom;
    const nextZoom = clamp(
      this.cameraZoom * (event.deltaY < 0 ? zoomStep : 1 / zoomStep),
      minCameraZoom,
      maxCameraZoom,
    );
    if (nextZoom === this.cameraZoom || previousScale <= 0) {
      return;
    }

    const worldX = (event.global.x - this.world.position.x) / previousScale;
    const worldY = (event.global.y - this.world.position.y) / previousScale;
    this.cameraZoom = nextZoom;

    const scale = this.fitScale * this.cameraZoom;
    const centered = this.centeredWorldRect(scale);
    this.cameraPanX = event.global.x - worldX * scale - centered.x;
    this.cameraPanY = event.global.y - worldY * scale - centered.y;
    this.applyWorldTransform();
    event.stopPropagation();
  }

  private applyWorldTransform(): void {
    if (this.viewportWidth < 1 || this.viewportHeight < 1) {
      return;
    }

    const availableHeight = Math.max(
      this.viewportHeight - worldLayout.hudHeight,
      260,
    );
    this.fitScale = Math.min(
      (this.viewportWidth - worldLayout.padding * 2) / this.mapWidth,
      (availableHeight - worldLayout.padding * 2) / this.mapHeight,
    );

    const scale = this.fitScale * this.cameraZoom;
    const centered = this.centeredWorldRect(scale);
    const clampedPan = this.clampedPan(
      centered.x,
      centered.y,
      centered.width,
      centered.height,
    );
    this.cameraPanX = clampedPan.x;
    this.cameraPanY = clampedPan.y;
    this.world.position.set(
      centered.x + this.cameraPanX,
      centered.y + this.cameraPanY,
    );
    this.world.scale.set(scale);
  }

  private centeredWorldRect(scale: number): {
    x: number;
    y: number;
    width: number;
    height: number;
  } {
    const availableHeight = Math.max(
      this.viewportHeight - worldLayout.hudHeight,
      260,
    );
    const scaledWidth = this.mapWidth * scale;
    const scaledHeight = this.mapHeight * scale;

    return {
      x: (this.viewportWidth - scaledWidth) * 0.5,
      y: worldLayout.hudHeight + (availableHeight - scaledHeight) * 0.5,
      width: scaledWidth,
      height: scaledHeight,
    };
  }

  private clampedPan(
    centeredX: number,
    centeredY: number,
    scaledWidth: number,
    scaledHeight: number,
  ): { x: number; y: number } {
    const viewportLeft = worldLayout.padding;
    const viewportRight = this.viewportWidth - worldLayout.padding;
    const viewportTop = worldLayout.hudHeight + worldLayout.padding;
    const viewportBottom = this.viewportHeight - worldLayout.padding;
    const viewportWidth = viewportRight - viewportLeft;
    const viewportHeight = viewportBottom - viewportTop;

    if (scaledWidth <= viewportWidth) {
      this.cameraPanX = 0;
    }
    if (scaledHeight <= viewportHeight) {
      this.cameraPanY = 0;
    }

    const positionX = centeredX + this.cameraPanX;
    const positionY = centeredY + this.cameraPanY;
    const minX = viewportRight - scaledWidth;
    const maxX = viewportLeft;
    const minY = viewportBottom - scaledHeight;
    const maxY = viewportTop;

    return {
      x:
        scaledWidth <= viewportWidth
          ? 0
          : clamp(positionX, minX, maxX) - centeredX,
      y:
        scaledHeight <= viewportHeight
          ? 0
          : clamp(positionY, minY, maxY) - centeredY,
    };
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

function clamp(value: number, minValue: number, maxValue: number): number {
  return Math.min(Math.max(value, minValue), maxValue);
}
