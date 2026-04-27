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
  PlayerColor,
  Snapshot,
} from "../../../game/protocol";

import { worldLayout, palette, resolveOwnershipColor } from "../theme";

import { FleetView } from "./FleetView";

import { PlanetView } from "./PlanetView";

interface Star {
  x: number;
  y: number;
  radius: number;
  alpha: number;
}

interface GameBoardCallbacks {
  onAdjustSendPercentage: (delta: number) => void;
  onClearSelection: () => void;
  onPlanetActivate: (planetID: number, additive: boolean) => void;
  onPlanetBoxSelect: (planetIDs: number[], additive: boolean) => void;
}

const initialShowDebugFleetTrails = false;
const minCameraZoom = 1;
const maxCameraZoom = 3.5;
const zoomStep = 1.15;
const panDragThreshold = 8;
const predictionLeadTicks = 1.35;
const spawnCameraZoom = 2.1;

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
  private readonly previewLayer = new Graphics();
  private readonly planetLayer = new Container();
  private readonly selectionOverlay = new Graphics();
  private readonly fleetViews = new Map<number, FleetView>();
  private readonly planetViews = new Map<number, PlanetView>();
  private readonly currentPlanets = new Map<number, PlanetSnapshot>();
  private readonly currentPlayerColors = new Map<number, number>();
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
  private hoveredPlanetID: number | null = null;
  private readonly selectedSourceIDs = new Set<number>();
  private shouldAutoFocusPlayer = true;
  private showDebugFleetTrails = initialShowDebugFleetTrails;
  private fleetPredictionMS = 0;
  private fleetPredictionLimitMS = 100;
  private activePointerButton: number | null = null;
  private panActive = false;
  private selectionBoxActive = false;
  private selectionAdditive = false;
  private dragStartX = 0;
  private dragStartY = 0;
  private dragLastX = 0;
  private dragLastY = 0;
  private dragCurrentX = 0;
  private dragCurrentY = 0;

  constructor(private readonly callbacks: GameBoardCallbacks) {
    super();

    this.eventMode = "static";
    this.world.addChild(
      this.worldBackdrop,
      this.fleetLayer,
      this.previewLayer,
      this.planetLayer,
    );
    this.addChild(this.screenBackdrop, this.world, this.selectionOverlay);
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
    selectedSourceIDs: ReadonlySet<number>,
  ): void {
    this.replaceSelectedSourceIDs(selectedSourceIDs);

    if (snapshot === null) {
      this.currentPlayerColors.clear();
      this.currentPlanets.clear();
      this.hoveredPlanetID = null;
      this.shouldAutoFocusPlayer = true;
      this.syncPlanets([], playerID, selectedSourceIDs);
      this.syncFleets([]);
      this.previousFleets.clear();
      this.previousPlanets.clear();
      this.fleetPredictionMS = 0;
      this.clearSelectionBox();
      this.drawPreviewPaths();
      return;
    }

    this.syncWorldBounds(snapshot.width, snapshot.height);
    this.syncPlayerColors(snapshot.playerColors);
    this.fleetPredictionMS = 0;
    this.fleetPredictionLimitMS = Math.max(
      50,
      (1000 / Math.max(1, snapshot.tickRate)) * predictionLeadTicks,
    );

    this.syncPlanets(snapshot.planets, playerID, selectedSourceIDs);
    this.syncAutoFocus(snapshot.planets, playerID);
    this.syncLandingImpacts(snapshot.fleets, playerID);
    this.syncFleets(snapshot.fleets);
    if (
      this.hoveredPlanetID !== null &&
      !this.currentPlanets.has(this.hoveredPlanetID)
    ) {
      this.hoveredPlanetID = null;
    }
    this.drawPreviewPaths();
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

  public setShowDebugFleetTrails(value: boolean): void {
    if (this.showDebugFleetTrails === value) {
      return;
    }

    this.showDebugFleetTrails = value;
    for (const fleetView of this.fleetViews.values()) {
      fleetView.setShowDebugTrail(value);
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
    if (event.button !== 0 && event.button !== 2) {
      return;
    }

    if (event.button === 2 && event.global.y < worldLayout.hudHeight) {
      return;
    }

    this.activePointerButton = event.button;
    this.panActive = false;
    this.selectionBoxActive = false;
    this.selectionAdditive = this.isAdditiveSelection(event);
    this.dragStartX = event.global.x;
    this.dragStartY = event.global.y;
    this.dragLastX = event.global.x;
    this.dragLastY = event.global.y;
    this.dragCurrentX = event.global.x;
    this.dragCurrentY = event.global.y;

    if (event.button === 2) {
      this.cursor = "grabbing";
      event.stopPropagation();
    }
  }

  private handlePointerMove(event: FederatedPointerEvent): void {
    if (this.activePointerButton === null) {
      if (event.global.y < worldLayout.hudHeight) {
        this.setHoveredPlanet(null);
        return;
      }

      this.setHoveredPlanet(
        this.resolvePlanetAtScreen(event.global.x, event.global.y),
      );
      return;
    }

    const dragDistanceX = event.global.x - this.dragStartX;
    const dragDistanceY = event.global.y - this.dragStartY;
    const passedThreshold =
      dragDistanceX * dragDistanceX + dragDistanceY * dragDistanceY >=
      panDragThreshold * panDragThreshold;

    if (this.activePointerButton === 2) {
      if (!this.panActive) {
        if (!passedThreshold) {
          return;
        }
        this.panActive = true;
      }

      this.cameraPanX += event.global.x - this.dragLastX;
      this.cameraPanY += event.global.y - this.dragLastY;
      this.dragLastX = event.global.x;
      this.dragLastY = event.global.y;
      this.applyWorldTransform();
      event.stopPropagation();
      return;
    }

    if (!this.selectionBoxActive && !passedThreshold) {
      return;
    }

    this.selectionBoxActive = true;
    this.dragCurrentX = event.global.x;
    this.dragCurrentY = event.global.y;
    this.drawSelectionBox();
    event.stopPropagation();
  }

  private handlePointerUp(event: FederatedPointerEvent): void {
    if (this.activePointerButton === null) {
      return;
    }

    const activeButton = this.activePointerButton;
    const didPan = this.panActive;
    const didSelect = this.selectionBoxActive;
    const additive = this.selectionAdditive;
    this.activePointerButton = null;
    this.panActive = false;
    this.selectionBoxActive = false;
    this.selectionAdditive = false;
    this.cursor = "default";

    if (activeButton === 2) {
      if (didPan) {
        event.stopPropagation();
      }
      return;
    }

    if (didSelect) {
      this.dragCurrentX = event.global.x;
      this.dragCurrentY = event.global.y;
      this.callbacks.onPlanetBoxSelect(
        this.resolvePlanetsInSelectionBox(),
        additive,
      );
      this.clearSelectionBox();
      event.stopPropagation();
      return;
    }

    if (this.dragStartY < worldLayout.hudHeight) {
      this.setHoveredPlanet(null);
      return;
    }

    const planetID = this.resolvePlanetAtScreen(event.global.x, event.global.y);
    if (planetID !== null) {
      this.setHoveredPlanet(planetID);
      this.callbacks.onPlanetActivate(planetID, additive);
      event.stopPropagation();
      return;
    }

    this.setHoveredPlanet(null);
    if (!additive) {
      this.callbacks.onClearSelection();
      event.stopPropagation();
    }
  }

  private drawSelectionBox(): void {
    const left = Math.min(this.dragStartX, this.dragCurrentX);
    const top = Math.max(
      worldLayout.hudHeight,
      Math.min(this.dragStartY, this.dragCurrentY),
    );
    const width = Math.abs(this.dragCurrentX - this.dragStartX);
    const bottom = Math.max(
      worldLayout.hudHeight,
      Math.max(this.dragStartY, this.dragCurrentY),
    );
    const height = bottom - top;

    this.selectionOverlay.clear();
    this.selectionOverlay.roundRect(left, top, width, height, 10);
    this.selectionOverlay.fill({ color: palette.friendly, alpha: 0.12 });
    this.selectionOverlay.roundRect(left, top, width, height, 10);
    this.selectionOverlay.stroke({
      color: palette.friendly,
      width: 2,
      alpha: 0.82,
    });
  }

  private clearSelectionBox(): void {
    this.selectionOverlay.clear();
  }

  private resolvePlanetsInSelectionBox(): number[] {
    const left = Math.min(this.dragStartX, this.dragCurrentX);
    const right = Math.max(this.dragStartX, this.dragCurrentX);
    const top = Math.max(
      worldLayout.hudHeight,
      Math.min(this.dragStartY, this.dragCurrentY),
    );
    const bottom = Math.max(this.dragStartY, this.dragCurrentY);
    const scale = this.world.scale.x;
    const selectedIDs: number[] = [];

    for (const planet of this.currentPlanets.values()) {
      const screenX = this.world.position.x + planet.x * scale;
      const screenY = this.world.position.y + planet.y * scale;
      if (
        screenX >= left &&
        screenX <= right &&
        screenY >= top &&
        screenY <= bottom
      ) {
        selectedIDs.push(planet.id);
      }
    }

    selectedIDs.sort((first, second) => first - second);
    return selectedIDs;
  }

  private resolvePlanetAtScreen(x: number, y: number): number | null {
    const scale = this.world.scale.x;
    if (scale <= 0) {
      return null;
    }

    const worldX = (x - this.world.position.x) / scale;
    const worldY = (y - this.world.position.y) / scale;
    let bestPlanetID: number | null = null;
    let bestDistanceSquared = Number.POSITIVE_INFINITY;

    for (const planet of this.currentPlanets.values()) {
      const dx = worldX - planet.x;
      const dy = worldY - planet.y;
      const distanceSquared = dx * dx + dy * dy;
      if (distanceSquared > planet.r * planet.r) {
        continue;
      }
      if (distanceSquared < bestDistanceSquared) {
        bestDistanceSquared = distanceSquared;
        bestPlanetID = planet.id;
      }
    }

    return bestPlanetID;
  }

  private isAdditiveSelection(event: FederatedPointerEvent): boolean {
    return event.ctrlKey || event.metaKey;
  }

  private handleWheel(event: FederatedWheelEvent): void {
    if (this.viewportWidth < 1 || this.viewportHeight < 1) {
      return;
    }
    if (event.global.y < worldLayout.hudHeight) {
      return;
    }

    if (event.shiftKey) {
      this.callbacks.onAdjustSendPercentage(event.deltaY < 0 ? 10 : -10);
      event.stopPropagation();
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
    this.drawPreviewPaths();
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
    selectedSourceIDs: ReadonlySet<number>,
  ): void {
    const activeIDs = new Set<number>();
    this.currentPlanets.clear();

    for (const planet of planets) {
      activeIDs.add(planet.id);
      this.currentPlanets.set(planet.id, planet);
      let view = this.planetViews.get(planet.id);
      if (view === undefined) {
        view = new PlanetView();
        this.planetViews.set(planet.id, view);
        this.planetLayer.addChild(view);
      }

      view.sync({
        planet,
        selected: selectedSourceIDs.has(planet.id),
        color: this.resolveOwnerColor(planet.owner),
        concealShips: this.shouldConcealShips(planet.owner, playerID),
        neutral: planet.owner === 0,
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

  private replaceSelectedSourceIDs(
    selectedSourceIDs: ReadonlySet<number>,
  ): void {
    this.selectedSourceIDs.clear();
    for (const planetID of selectedSourceIDs) {
      this.selectedSourceIDs.add(planetID);
    }
  }

  private syncPlayerColors(playerColors: PlayerColor[]): void {
    this.currentPlayerColors.clear();
    for (const playerColor of playerColors) {
      this.currentPlayerColors.set(playerColor.playerId, playerColor.color);
    }
  }

  private syncAutoFocus(
    planets: PlanetSnapshot[],
    playerID: number | null,
  ): void {
    if (
      !this.shouldAutoFocusPlayer ||
      playerID === null ||
      planets.length === 0
    ) {
      return;
    }

    const spawnPlanet = this.resolveSpawnPlanet(planets, playerID);
    if (spawnPlanet === null) {
      return;
    }

    this.cameraZoom = spawnCameraZoom;
    this.focusPlanet(spawnPlanet);
    this.shouldAutoFocusPlayer = false;
  }

  private resolveSpawnPlanet(
    planets: PlanetSnapshot[],
    playerID: number,
  ): PlanetSnapshot | null {
    let bestPlanet: PlanetSnapshot | null = null;

    for (const planet of planets) {
      if (planet.owner !== playerID) {
        continue;
      }

      if (
        bestPlanet === null ||
        planet.ships > bestPlanet.ships ||
        (planet.ships === bestPlanet.ships && planet.r > bestPlanet.r) ||
        (planet.ships === bestPlanet.ships &&
          planet.r === bestPlanet.r &&
          planet.id < bestPlanet.id)
      ) {
        bestPlanet = planet;
      }
    }

    return bestPlanet;
  }

  private focusPlanet(planet: PlanetSnapshot): void {
    if (this.viewportWidth < 1 || this.viewportHeight < 1) {
      return;
    }

    const scale = this.fitScale * this.cameraZoom;
    if (scale <= 0) {
      return;
    }

    const centered = this.centeredWorldRect(scale);
    const targetX = this.viewportWidth * 0.5;
    const targetY =
      worldLayout.hudHeight +
      (this.viewportHeight - worldLayout.hudHeight) * 0.5;
    this.cameraPanX = targetX - centered.x - planet.x * scale;
    this.cameraPanY = targetY - centered.y - planet.y * scale;
    this.applyWorldTransform();
  }

  private setHoveredPlanet(planetID: number | null): void {
    if (this.hoveredPlanetID === planetID) {
      return;
    }

    this.hoveredPlanetID = planetID;
    this.drawPreviewPaths();
  }

  private drawPreviewPaths(): void {
    this.previewLayer.clear();
    if (this.hoveredPlanetID === null || this.selectedSourceIDs.size === 0) {
      return;
    }

    const destination = this.currentPlanets.get(this.hoveredPlanetID);
    if (destination === undefined) {
      return;
    }

    let drewAny = false;
    for (const sourceID of this.selectedSourceIDs) {
      if (sourceID === this.hoveredPlanetID) {
        continue;
      }

      const source = this.currentPlanets.get(sourceID);
      if (source === undefined) {
        continue;
      }

      const dx = destination.x - source.x;
      const dy = destination.y - source.y;
      const distance = Math.hypot(dx, dy);
      if (distance <= 0.001) {
        continue;
      }

      const directionX = dx / distance;
      const directionY = dy / distance;
      const startX = source.x + directionX * (source.r + 8);
      const startY = source.y + directionY * (source.r + 8);
      const endX = destination.x - directionX * (destination.r + 8);
      const endY = destination.y - directionY * (destination.r + 8);
      const color = this.resolveOwnerColor(source.owner);

      this.previewLayer.moveTo(startX, startY);
      this.previewLayer.lineTo(endX, endY);
      this.previewLayer.stroke({ color, width: 3, alpha: 0.84 });

      this.previewLayer.circle(startX, startY, 4.5);
      this.previewLayer.fill({ color, alpha: 0.55 });
      this.previewLayer.circle(endX, endY, 5.5);
      this.previewLayer.stroke({ color, width: 2, alpha: 0.92 });
      drewAny = true;
    }

    if (!drewAny) {
      this.previewLayer.clear();
    }
  }

  private syncFleets(fleets: FleetSnapshot[]): void {
    const activeIDs = new Set<number>();

    for (const fleet of fleets) {
      activeIDs.add(fleet.id);
      let view = this.fleetViews.get(fleet.id);
      if (view === undefined) {
        view = new FleetView(this.showDebugFleetTrails);
        this.fleetViews.set(fleet.id, view);
        this.fleetLayer.addChild(view);
      }

      view.setShowDebugTrail(this.showDebugFleetTrails);
      view.sync(fleet, this.resolveOwnerColor(fleet.owner));
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

  private shouldConcealShips(owner: number, playerID: number | null): boolean {
    return owner !== 0 && (playerID === null || owner !== playerID);
  }

  private resolveOwnerColor(owner: number): number {
    return resolveOwnershipColor(owner, this.currentPlayerColors);
  }
}

function clamp(value: number, minValue: number, maxValue: number): number {
  return Math.min(Math.max(value, minValue), maxValue);
}
