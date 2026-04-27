import { Container, Graphics, Text } from "pixi.js";

import type { ConnectionStatus } from "../../../game/GameClient";
import type { LobbyStatus, LobbySummary } from "../../../game/protocol";

import { palette } from "../theme";

import { HudButton } from "./HudButton";

const shipLength = 9;
const shipHalfWidth = 4.8;

interface BattlePlanet {
  x: number;
  y: number;
  radius: number;
  tint: number;
  orbitRadius: number;
  orbitSpeed: number;
  orbitTilt: number;
  squadSize: number;
  phase: number;
}

interface BattleLane {
  fromIndex: number;
  toIndex: number;
  tint: number;
  speed: number;
  phase: number;
  squadSize: number;
  sway: number;
}

interface OrbitShipMarker {
  node: Graphics;
  planetIndex: number;
  shipIndex: number;
}

interface AttackShipMarker {
  node: Graphics;
  lane: BattleLane;
  shipIndex: number;
}

interface ImpactRingMarker {
  node: Graphics;
  lane: BattleLane;
}

interface LobbyPanelCallbacks {
  onPlay: () => void;
}

export interface LobbyPanelModel {
  connectionStatus: ConnectionStatus;
  errorMessage: string | null;
  joinedLobbyId: string | null;
  lobbyStatus: LobbyStatus | null;
  lobbyPlayers: number;
  countdownMs: number | null;
  lobbies: LobbySummary[];
}

export class LobbyPanel extends Container {
  private static readonly viewportInset = 16;
  private static readonly statusWidthPadding = 84;
  private static readonly errorWidthPadding = 84;

  private readonly backdrop = new Graphics();
  private readonly stars = new Graphics();
  private readonly battleStatic = new Graphics();
  private readonly battleDynamic = new Container();
  private readonly panelGlow = new Graphics();
  private readonly panel = new Graphics();
  private readonly headerBand = new Graphics();
  private readonly listFrame = new Graphics();
  private readonly titleKicker = new Text({
    text: "",
    anchor: 0.5,
    style: {
      fill: palette.accent,
      fontFamily: "Trebuchet MS",
      fontSize: 13,
      fontWeight: "700",
      letterSpacing: 3,
    },
  });
  private readonly title = new Text({
    text: "Galact.IO",
    anchor: 0.5,
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 42,
      fontWeight: "700",
    },
  });
  private readonly subtitle = new Text({
    text: "",
    anchor: 0.5,
    style: {
      fill: 0xb7cade,
      fontFamily: "Trebuchet MS",
      fontSize: 17,
      fontWeight: "600",
      align: "center",
    },
  });
  private readonly status = new Text({
    anchor: 0.5,
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 17,
      fontWeight: "600",
      align: "center",
    },
  });
  private readonly error = new Text({
    anchor: 0.5,
    style: {
      fill: palette.warning,
      fontFamily: "Trebuchet MS",
      fontSize: 15,
      fontWeight: "600",
      align: "center",
    },
  });
  private readonly queueState = new Text({
    text: "",
    anchor: 0.5,
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 18,
      fontWeight: "600",
      align: "center",
    },
  });
  private readonly playButton: HudButton;
  private currentModel: LobbyPanelModel | null = null;
  private readonly battleLanes: BattleLane[] = [
    {
      fromIndex: 0,
      toIndex: 1,
      tint: palette.enemy,
      speed: 0.00018,
      phase: 0.1,
      squadSize: 4,
      sway: 18,
    },
    {
      fromIndex: 1,
      toIndex: 2,
      tint: palette.accent,
      speed: 0.00014,
      phase: 0.44,
      squadSize: 3,
      sway: 14,
    },
    {
      fromIndex: 2,
      toIndex: 0,
      tint: palette.friendly,
      speed: 0.00016,
      phase: 0.71,
      squadSize: 4,
      sway: 20,
    },
  ];
  private battlePlanets: BattlePlanet[] = [];
  private readonly orbitShips: OrbitShipMarker[] = [];
  private readonly attackShips: AttackShipMarker[] = [];
  private readonly impactRings: ImpactRingMarker[] = [];
  private viewportWidth = 0;
  private viewportHeight = 0;
  private panelX = 0;
  private panelY = 0;
  private panelWidth = 0;
  private panelHeight = 0;
  private headerHeight = 0;
  private footerHeight = 0;
  private listX = 0;
  private listY = 0;
  private listWidth = 0;
  private listHeight = 0;
  private compact = false;
  private animationTime = 0;

  constructor(callbacks: LobbyPanelCallbacks) {
    super();

    this.playButton = new HudButton({
      label: "Play",
      width: 172,
      height: 52,
      tint: 0x144466,
      onPress: callbacks.onPlay,
    });

    this.addChild(
      this.backdrop,
      this.stars,
      this.battleStatic,
      this.battleDynamic,
      this.panelGlow,
      this.panel,
      this.headerBand,
      this.listFrame,
      this.titleKicker,
      this.title,
      this.subtitle,
      this.status,
      this.error,
      this.queueState,
      this.playButton,
    );
  }

  public resize(width: number, height: number): void {
    this.viewportWidth = width;
    this.viewportHeight = height;
    this.compact = width < 920 || height < 720;

    const horizontalPadding = Math.max(20, Math.min(width * 0.06, 72));
    const verticalPadding = Math.max(20, Math.min(height * 0.08, 72));
    const preferredWidth = Math.min(
      980,
      Math.max(280, width - horizontalPadding * 2),
    );
    const preferredHeight = Math.min(
      760,
      Math.max(400, height - verticalPadding * 2),
    );
    const maxPanelWidth = Math.max(1, width - LobbyPanel.viewportInset);
    const maxPanelHeight = Math.max(1, height - LobbyPanel.viewportInset);

    this.panelWidth = Math.min(preferredWidth, maxPanelWidth);
    this.panelHeight = Math.min(preferredHeight, maxPanelHeight);
    this.panelX = Math.max(8, (width - this.panelWidth) * 0.5);
    this.panelY = Math.max(8, (height - this.panelHeight) * 0.5);

    this.title.style.fontSize = this.compact ? 34 : 42;
    this.subtitle.style.fontSize = this.compact ? 15 : 17;
    this.status.style.fontSize = this.compact ? 15 : 17;
    this.error.style.fontSize = this.compact ? 14 : 15;
    this.queueState.style.fontSize = this.compact ? 16 : 18;

    const listInset = Math.min(
      this.compact ? 20 : 28,
      Math.max(8, this.panelWidth * 0.08),
    );
    const preferredHeaderHeight = this.compact ? 188 : 214;
    const preferredFooterHeight = this.compact ? 24 : 30;
    const reservedHeight = Math.min(
      preferredHeaderHeight + preferredFooterHeight,
      Math.max(1, this.panelHeight - 8),
    );
    const chromeScale =
      reservedHeight / (preferredHeaderHeight + preferredFooterHeight);

    this.headerHeight = preferredHeaderHeight * chromeScale;
    this.footerHeight = preferredFooterHeight * chromeScale;

    this.listX = this.panelX + listInset;
    this.listY = this.panelY + this.headerHeight;
    this.listWidth = Math.max(1, this.panelWidth - listInset * 2);
    this.listHeight = Math.max(
      1,
      this.panelHeight - this.headerHeight - this.footerHeight,
    );
    this.battlePlanets = this.buildPlanets();

    this.drawBackdrop();
    this.drawStars();
    this.drawPanel();
    this.drawBattleStatic();
    this.rebuildBattleMarkers();
    this.updateBattleAnimation();

    this.title.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + (this.compact ? 90 : 100),
    );
    this.titleKicker.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + (this.compact ? 34 : 38),
    );
    this.subtitle.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + (this.compact ? 106 : 122),
    );
    this.status.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + (this.compact ? 138 : 156),
    );
    this.error.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + (this.compact ? 166 : 186),
    );
    this.queueState.position.set(
      this.listX + this.listWidth * 0.5,
      this.listY + this.listHeight * 0.4,
    );
    this.playButton.position.set(
      this.listX + this.listWidth * 0.5,
      this.listY + this.listHeight * 0.62,
    );

    this.subtitle.style.wordWrap = true;
    this.subtitle.style.wordWrapWidth = Math.max(180, this.panelWidth - 120);
    this.status.style.wordWrap = true;
    this.status.style.wordWrapWidth = Math.max(
      180,
      this.panelWidth - LobbyPanel.statusWidthPadding,
    );
    this.error.style.wordWrap = true;
    this.error.style.wordWrapWidth = Math.max(
      180,
      this.panelWidth - LobbyPanel.errorWidthPadding,
    );
    this.queueState.style.wordWrap = true;
    this.queueState.style.wordWrapWidth = Math.max(180, this.listWidth - 72);

    if (this.currentModel !== null) {
      this.render(this.currentModel);
    }
  }

  public update(deltaMS: number): void {
    this.animationTime += deltaMS;
    if (
      !this.visible ||
      this.viewportWidth === 0 ||
      this.viewportHeight === 0
    ) {
      return;
    }

    this.updateBattleAnimation();
  }

  public render(model: LobbyPanelModel): void {
    this.currentModel = model;
    this.status.text = this.buildStatus(model);
    this.status.style.fill = this.statusColor(model.connectionStatus);
    this.status.visible = this.status.text.length > 0;
    this.error.text = model.errorMessage ?? "";
    this.error.visible = model.errorMessage !== null;
    this.queueState.text = this.buildQueueState(model);
    this.queueState.visible = this.queueState.text.length > 0;
    this.playButton.setDisabled(
      model.connectionStatus !== "open" || model.lobbyStatus === "countdown",
    );
  }

  private drawBackdrop(): void {
    this.backdrop.clear();
    this.backdrop.rect(0, 0, this.viewportWidth, this.viewportHeight);
    this.backdrop.fill({ color: palette.background, alpha: 1 });

    const glowRadius = Math.max(this.viewportWidth, this.viewportHeight) * 0.32;
    this.backdrop.circle(
      this.viewportWidth * 0.12,
      this.viewportHeight * 0.16,
      glowRadius,
    );
    this.backdrop.fill({ color: palette.friendly, alpha: 0.08 });
    this.backdrop.circle(
      this.viewportWidth * 0.88,
      this.viewportHeight * 0.2,
      glowRadius * 0.9,
    );
    this.backdrop.fill({ color: palette.enemy, alpha: 0.06 });
    this.backdrop.circle(
      this.viewportWidth * 0.76,
      this.viewportHeight * 0.84,
      glowRadius * 0.7,
    );
    this.backdrop.fill({ color: palette.accent, alpha: 0.05 });
  }

  private drawStars(): void {
    this.stars.clear();
    const starCount = Math.max(
      48,
      Math.floor((this.viewportWidth + this.viewportHeight) / 26),
    );

    for (let index = 0; index < starCount; index += 1) {
      const x = this.seed(index * 13.41) * this.viewportWidth;
      const y = this.seed(index * 21.73) * this.viewportHeight;
      const radius = 0.7 + this.seed(index * 9.17) * 1.8;
      const alpha = 0.18 + this.seed(index * 5.19) * 0.48;
      this.stars.circle(x, y, radius);
      this.stars.fill({ color: palette.text, alpha });
    }
  }

  private drawPanel(): void {
    this.panelGlow.clear();
    this.panelGlow.roundRect(
      this.panelX - 6,
      this.panelY - 6,
      this.panelWidth + 12,
      this.panelHeight + 12,
      34,
    );
    this.panelGlow.fill({ color: palette.friendly, alpha: 0.08 });
    this.panelGlow.roundRect(
      this.panelX - 10,
      this.panelY - 10,
      this.panelWidth + 20,
      this.panelHeight + 20,
      38,
    );
    this.panelGlow.stroke({ color: palette.friendly, width: 1, alpha: 0.18 });

    this.panel.clear();
    this.panel.roundRect(
      this.panelX,
      this.panelY,
      this.panelWidth,
      this.panelHeight,
      28,
    );
    this.panel.fill({ color: palette.panel, alpha: 0.78 });
    this.panel.roundRect(
      this.panelX,
      this.panelY,
      this.panelWidth,
      this.panelHeight,
      28,
    );
    this.panel.stroke({ color: palette.outline, width: 2, alpha: 1 });

    this.headerBand.clear();
    this.headerBand.roundRect(
      this.panelX + 14,
      this.panelY + 14,
      Math.max(1, this.panelWidth - 28),
      Math.max(1, this.headerHeight - 22),
      22,
    );
    this.headerBand.fill({ color: palette.surfaceAlt, alpha: 0.42 });
    this.headerBand.roundRect(
      this.listX,
      this.listY - 12,
      this.listWidth,
      Math.max(1, this.listHeight + 12),
      24,
    );
    this.headerBand.stroke({ color: palette.outline, width: 1, alpha: 0.72 });

    this.listFrame.clear();
    this.listFrame.roundRect(
      this.listX,
      this.listY,
      this.listWidth,
      Math.max(1, this.listHeight),
      24,
    );
    this.listFrame.fill({ color: palette.surface, alpha: 0.58 });

    this.listFrame.roundRect(
      this.listX + 22,
      this.listY + 22,
      Math.max(1, this.listWidth - 44),
      Math.max(1, this.listHeight - 44),
      20,
    );
    this.listFrame.stroke({ color: palette.outline, width: 1, alpha: 0.42 });
  }

  private drawBattleStatic(): void {
    this.battleStatic.clear();

    for (const planet of this.battlePlanets) {
      this.battleStatic.circle(planet.x, planet.y, planet.radius * 1.85);
      this.battleStatic.fill({ color: planet.tint, alpha: 0.08 });
      this.battleStatic.circle(planet.x, planet.y, planet.radius * 1.12);
      this.battleStatic.fill({ color: planet.tint, alpha: 0.2 });
      this.battleStatic.circle(planet.x, planet.y, planet.radius);
      this.battleStatic.fill({ color: 0x10283c, alpha: 0.96 });
      this.battleStatic.circle(planet.x, planet.y, planet.radius);
      this.battleStatic.stroke({ color: planet.tint, width: 2, alpha: 0.92 });
      this.battleStatic.circle(planet.x, planet.y, planet.orbitRadius);
      this.battleStatic.stroke({ color: planet.tint, width: 1.2, alpha: 0.22 });
    }

    for (const lane of this.battleLanes) {
      const from = this.battlePlanets[lane.fromIndex];
      const to = this.battlePlanets[lane.toIndex];
      const dx = to.x - from.x;
      const dy = to.y - from.y;
      const distance = Math.hypot(dx, dy) || 1;
      const beamEndX = to.x - (dx / distance) * to.radius * 0.8;
      const beamEndY = to.y - (dy / distance) * to.radius * 0.8;

      this.battleStatic.moveTo(from.x, from.y);
      this.battleStatic.lineTo(beamEndX, beamEndY);
      this.battleStatic.stroke({
        color: lane.tint,
        width: 1.4,
        alpha: 0.16,
      });
    }
  }

  private buildPlanets(): BattlePlanet[] {
    const baseRadius = Math.max(
      28,
      Math.min(this.viewportWidth, this.viewportHeight) * 0.055,
    );
    const leftX = Math.max(baseRadius * 2.6, this.viewportWidth * 0.18);
    const upperY = Math.max(baseRadius * 2.4, this.viewportHeight * 0.24);
    const rightX = Math.min(
      this.viewportWidth - baseRadius * 2.6,
      this.viewportWidth * 0.82,
    );
    const lowerY = Math.min(
      this.viewportHeight - baseRadius * 2.8,
      this.viewportHeight * 0.74,
    );

    return [
      {
        x: leftX,
        y: upperY,
        radius: baseRadius,
        tint: palette.friendly,
        orbitRadius: baseRadius * 1.9,
        orbitSpeed: 0.0012,
        orbitTilt: 0.62,
        squadSize: 4,
        phase: 0.3,
      },
      {
        x: rightX,
        y: Math.max(baseRadius * 2.2, this.viewportHeight * 0.22),
        radius: baseRadius * 0.92,
        tint: palette.enemy,
        orbitRadius: baseRadius * 1.7,
        orbitSpeed: -0.001,
        orbitTilt: 0.68,
        squadSize: 5,
        phase: 1.1,
      },
      {
        x: Math.min(
          this.viewportWidth - baseRadius * 2.4,
          this.viewportWidth * 0.78,
        ),
        y: lowerY,
        radius: baseRadius * 1.08,
        tint: palette.accent,
        orbitRadius: baseRadius * 2.05,
        orbitSpeed: 0.00085,
        orbitTilt: 0.58,
        squadSize: 4,
        phase: 2.4,
      },
    ];
  }

  private rebuildBattleMarkers(): void {
    for (const marker of this.orbitShips) {
      marker.node.destroy();
    }
    for (const marker of this.attackShips) {
      marker.node.destroy();
    }
    for (const ring of this.impactRings) {
      ring.node.destroy();
    }

    this.orbitShips.length = 0;
    this.attackShips.length = 0;
    this.impactRings.length = 0;
    this.battleDynamic.removeChildren();

    this.battlePlanets.forEach((planet, planetIndex) => {
      for (let shipIndex = 0; shipIndex < planet.squadSize; shipIndex += 1) {
        const node = new Graphics();
        this.drawShipGlyph(node, planet.tint, 1);
        this.orbitShips.push({ node, planetIndex, shipIndex });
        this.battleDynamic.addChild(node);
      }
    });

    for (const lane of this.battleLanes) {
      const ring = new Graphics();
      ring.circle(0, 0, 1);
      ring.stroke({ color: lane.tint, width: 1.4, alpha: 1 });
      this.impactRings.push({ node: ring, lane });
      this.battleDynamic.addChild(ring);

      for (let shipIndex = 0; shipIndex < lane.squadSize; shipIndex += 1) {
        const node = new Graphics();
        this.drawShipGlyph(node, lane.tint, 0.9);
        this.attackShips.push({ node, lane, shipIndex });
        this.battleDynamic.addChild(node);
      }
    }
  }

  private updateBattleAnimation(): void {
    const time = this.animationTime;

    for (const marker of this.orbitShips) {
      const planet = this.battlePlanets[marker.planetIndex];
      const angle =
        time * planet.orbitSpeed +
        marker.shipIndex * ((Math.PI * 2) / planet.squadSize) +
        planet.phase;
      marker.node.position.set(
        planet.x + Math.cos(angle) * planet.orbitRadius,
        planet.y + Math.sin(angle) * planet.orbitRadius * planet.orbitTilt,
      );
      marker.node.rotation = angle + Math.PI * 0.5;
    }

    for (const marker of this.attackShips) {
      const from = this.battlePlanets[marker.lane.fromIndex];
      const to = this.battlePlanets[marker.lane.toIndex];
      const dx = to.x - from.x;
      const dy = to.y - from.y;
      const distance = Math.hypot(dx, dy) || 1;
      const normalX = -dy / distance;
      const normalY = dx / distance;
      const progress =
        (time * marker.lane.speed +
          marker.lane.phase +
          marker.shipIndex / marker.lane.squadSize) %
        1;

      marker.node.position.set(
        from.x +
          dx * progress +
          normalX *
            Math.sin(progress * 9 + marker.lane.phase * 7) *
            marker.lane.sway,
        from.y +
          dy * progress +
          normalY *
            Math.sin(progress * 9 + marker.lane.phase * 7) *
            marker.lane.sway,
      );
      marker.node.rotation = Math.atan2(dy, dx);
    }

    for (const ring of this.impactRings) {
      const from = this.battlePlanets[ring.lane.fromIndex];
      const to = this.battlePlanets[ring.lane.toIndex];
      const dx = to.x - from.x;
      const dy = to.y - from.y;
      const distance = Math.hypot(dx, dy) || 1;
      const beamEndX = to.x - (dx / distance) * to.radius * 0.8;
      const beamEndY = to.y - (dy / distance) * to.radius * 0.8;
      const impactPulse =
        0.55 +
        0.45 * Math.sin(this.animationTime * 0.004 + ring.lane.phase * 8);

      ring.node.position.set(beamEndX, beamEndY);
      ring.node.scale.set(to.radius * (0.2 + impactPulse * 0.24));
      ring.node.alpha = 0.18 + impactPulse * 0.18;
    }
  }

  private drawShipGlyph(node: Graphics, tint: number, alpha: number): void {
    node.clear();
    node.moveTo(shipLength, 0);
    node.lineTo(-shipLength * 0.45, -shipHalfWidth);
    node.lineTo(-shipLength * 0.9, 0);
    node.lineTo(-shipLength * 0.45, shipHalfWidth);
    node.closePath();
    node.fill({ color: tint, alpha });
  }

  private buildStatus(model: LobbyPanelModel): string {
    if (model.joinedLobbyId === null || model.lobbyStatus === null) {
      return "";
    }

    const countdownLabel =
      model.lobbyStatus === "countdown" && model.countdownMs !== null
        ? ` · starts in ${Math.max(1, Math.ceil(model.countdownMs / 1000))}s`
        : "";

    return `Joined ${model.joinedLobbyId} · ${model.lobbyPlayers} players · ${model.lobbyStatus}${countdownLabel}`;
  }

  private buildQueueState(model: LobbyPanelModel): string {
    if (model.joinedLobbyId !== null && model.lobbyStatus !== null) {
      const countdownLabel =
        model.lobbyStatus === "countdown" && model.countdownMs !== null
          ? ` Match starts in ${Math.max(1, Math.ceil(model.countdownMs / 1000))}s.`
          : "";
      return `Queued in ${model.joinedLobbyId.toUpperCase()} with ${model.lobbyPlayers} pilot${model.lobbyPlayers === 1 ? "" : "s"}.${countdownLabel}`;
    }

    return "Press play to enter the next available match queue.";
  }

  private statusColor(status: ConnectionStatus): number {
    switch (status) {
      case "open":
        return palette.friendly;
      case "connecting":
        return palette.accent;
      case "closed":
        return palette.enemy;
      default:
        return palette.mutedText;
    }
  }

  private seed(value: number): number {
    return Math.abs(Math.sin(value * 12.9898) * 43758.5453) % 1;
  }
}
