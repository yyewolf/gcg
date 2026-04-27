import { Container, Graphics, Text } from "pixi.js";

import type { ConnectionStatus } from "../../../game/GameClient";
import type { LobbyStatus, LobbySummary } from "../../../game/protocol";

import { palette } from "../theme";

import { HudButton } from "./HudButton";

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

interface LobbyPanelCallbacks {
  onJoinLobby: (lobbyID: string) => void;
  onReconnect: () => void;
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
  private readonly battleScene = new Graphics();
  private readonly panelGlow = new Graphics();
  private readonly panel = new Graphics();
  private readonly headerBand = new Graphics();
  private readonly listFrame = new Graphics();
  private readonly titleKicker = new Text({
    text: "TACTICAL MATCHMAKING",
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
    text: "Fleet Lobby",
    anchor: 0.5,
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 42,
      fontWeight: "700",
    },
  });
  private readonly subtitle = new Text({
    text: "Join a live staging area while the sector map lights up around you.",
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
  private readonly emptyState = new Text({
    text: "No active lobbies right now. Hold position and wait for a new signal.",
    anchor: 0.5,
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 18,
      fontWeight: "600",
      align: "center",
    },
  });
  private readonly reconnectButton: HudButton;
  private readonly entryBackdrops = new Map<string, Graphics>();
  private readonly entryLabels = new Map<string, Text>();
  private readonly entryButtons = new Map<string, HudButton>();
  private currentModel: LobbyPanelModel | null = null;
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
  private rowHeight = 0;
  private rowButtonWidth = 0;
  private compact = false;
  private animationTime = 0;

  constructor(private readonly callbacks: LobbyPanelCallbacks) {
    super();

    this.reconnectButton = new HudButton({
      label: "Reconnect",
      width: 130,
      height: 42,
      tint: 0x163248,
      onPress: callbacks.onReconnect,
    });

    this.addChild(
      this.backdrop,
      this.stars,
      this.battleScene,
      this.panelGlow,
      this.panel,
      this.headerBand,
      this.listFrame,
      this.titleKicker,
      this.title,
      this.subtitle,
      this.status,
      this.error,
      this.emptyState,
      this.reconnectButton,
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

    this.rowButtonWidth = 108;

    this.title.style.fontSize = this.compact ? 34 : 42;
    this.subtitle.style.fontSize = this.compact ? 15 : 17;
    this.status.style.fontSize = this.compact ? 15 : 17;
    this.error.style.fontSize = this.compact ? 14 : 15;
    this.emptyState.style.fontSize = this.compact ? 16 : 18;

    const listInset = Math.min(
      this.compact ? 20 : 28,
      Math.max(8, this.panelWidth * 0.08),
    );
    const preferredHeaderHeight = this.compact ? 188 : 214;
    const preferredFooterHeight = this.compact ? 70 : 78;
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

    this.drawBackdrop();
    this.drawStars();
    this.drawPanel();
    this.drawBattleScene();

    this.title.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + (this.compact ? 68 : 80),
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
      this.panelY + (this.compact ? 142 : 160),
    );
    this.error.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + (this.compact ? 170 : 190),
    );
    this.emptyState.position.set(
      this.listX + this.listWidth * 0.5,
      this.listY + this.listHeight * 0.5,
    );
    this.reconnectButton.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + this.panelHeight - (this.compact ? 36 : 40),
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
    this.emptyState.style.wordWrap = true;
    this.emptyState.style.wordWrapWidth = Math.max(140, this.listWidth - 40);

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

    this.drawBattleScene();
  }

  public render(model: LobbyPanelModel): void {
    this.currentModel = model;
    this.status.text = this.buildStatus(model);
    this.status.style.fill = this.statusColor(model.connectionStatus);
    this.error.text = model.errorMessage ?? "";
    this.error.visible = model.errorMessage !== null;
    this.emptyState.visible = model.lobbies.length === 0;
    this.reconnectButton.setDisabled(model.connectionStatus === "connecting");
    this.syncEntries(model);
  }

  private syncEntries(model: LobbyPanelModel): void {
    const activeIDs = new Set<string>();
    const lobbyCount = Math.max(1, model.lobbies.length);
    const rowGap = this.compact ? 12 : 14;
    const totalGap = rowGap * Math.max(0, model.lobbies.length - 1);
    const availableHeight = Math.max(88, this.listHeight - 28 - totalGap);
    this.rowHeight = Math.max(
      this.compact ? 62 : 72,
      Math.min(this.compact ? 74 : 84, availableHeight / lobbyCount),
    );
    const startY = this.listY + 18;
    const labelX = this.listX + 22;
    const buttonX =
      this.listX + this.listWidth - 24 - this.rowButtonWidth * 0.5;
    const labelWidth = Math.max(1, this.listWidth - this.rowButtonWidth - 88);

    model.lobbies.forEach((lobby, index) => {
      activeIDs.add(lobby.id);
      let row = this.entryBackdrops.get(lobby.id);
      if (row === undefined) {
        row = new Graphics();
        this.entryBackdrops.set(lobby.id, row);
        this.addChildAt(row, this.getChildIndex(this.emptyState));
      }

      let label = this.entryLabels.get(lobby.id);
      if (label === undefined) {
        label = new Text({
          anchor: { x: 0, y: 0.5 },
          style: {
            fill: palette.text,
            fontFamily: "Trebuchet MS",
            fontSize: 17,
            fontWeight: "600",
            wordWrap: true,
          },
        });
        this.entryLabels.set(lobby.id, label);
        this.addChild(label);
      }

      let button = this.entryButtons.get(lobby.id);
      if (button === undefined) {
        button = new HudButton({
          label: "Join",
          width: this.rowButtonWidth,
          height: 40,
          onPress: () => this.callbacks.onJoinLobby(lobby.id),
        });
        this.entryButtons.set(lobby.id, button);
        this.addChild(button);
      }

      const rowY = startY + index * (this.rowHeight + rowGap);
      const accent = this.lobbyAccent(
        lobby.status,
        model.joinedLobbyId === lobby.id,
      );
      const rowWidth = Math.max(1, this.listWidth - 20);
      const accentHeight = Math.max(1, this.rowHeight - 20);

      row.clear();
      row.roundRect(this.listX + 10, rowY, rowWidth, this.rowHeight, 20);
      row.fill({ color: 0x081827, alpha: 0.84 });
      row.roundRect(this.listX + 10, rowY, rowWidth, this.rowHeight, 20);
      row.stroke({ color: accent, width: 1.5, alpha: 0.9 });
      row.roundRect(this.listX + 16, rowY + 10, 6, accentHeight, 4);
      row.fill({ color: accent, alpha: 0.92 });

      label.text = this.describeLobby(lobby, model.joinedLobbyId === lobby.id);
      label.style.fontSize = this.compact ? 15 : 17;
      label.style.wordWrapWidth = labelWidth;
      label.position.set(labelX + 10, rowY + this.rowHeight * 0.5);
      button.position.set(buttonX, rowY + this.rowHeight * 0.5);
      button.setDisabled(
        lobby.status === "playing" || model.joinedLobbyId === lobby.id,
      );
    });

    this.emptyState.visible = model.lobbies.length === 0;

    for (const [lobbyID, row] of this.entryBackdrops) {
      if (activeIDs.has(lobbyID)) {
        continue;
      }

      this.entryBackdrops.delete(lobbyID);
      row.destroy();
    }

    for (const [lobbyID, label] of this.entryLabels) {
      if (activeIDs.has(lobbyID)) {
        continue;
      }

      this.entryLabels.delete(lobbyID);
      label.destroy();
    }

    for (const [lobbyID, button] of this.entryButtons) {
      if (activeIDs.has(lobbyID)) {
        continue;
      }

      this.entryButtons.delete(lobbyID);
      button.destroy();
    }
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
  }

  private drawBattleScene(): void {
    this.battleScene.clear();

    const planets = this.buildPlanets();
    const time = this.animationTime;

    for (const planet of planets) {
      this.battleScene.circle(planet.x, planet.y, planet.radius * 1.85);
      this.battleScene.fill({ color: planet.tint, alpha: 0.08 });
      this.battleScene.circle(planet.x, planet.y, planet.radius * 1.12);
      this.battleScene.fill({ color: planet.tint, alpha: 0.2 });
      this.battleScene.circle(planet.x, planet.y, planet.radius);
      this.battleScene.fill({ color: 0x10283c, alpha: 0.96 });
      this.battleScene.circle(planet.x, planet.y, planet.radius);
      this.battleScene.stroke({ color: planet.tint, width: 2, alpha: 0.92 });
      this.battleScene.circle(planet.x, planet.y, planet.orbitRadius);
      this.battleScene.stroke({ color: planet.tint, width: 1.2, alpha: 0.22 });

      for (let shipIndex = 0; shipIndex < planet.squadSize; shipIndex += 1) {
        const angle =
          time * planet.orbitSpeed +
          shipIndex * ((Math.PI * 2) / planet.squadSize) +
          planet.phase;
        const orbitX = planet.x + Math.cos(angle) * planet.orbitRadius;
        const orbitY =
          planet.y + Math.sin(angle) * planet.orbitRadius * planet.orbitTilt;
        const heading = angle + Math.PI * 0.5;
        this.drawShip(orbitX, orbitY, heading, planet.tint, 1);
      }
    }

    this.drawAttackLane(
      planets[0],
      planets[1],
      palette.enemy,
      0.00018,
      0.1,
      4,
      18,
    );
    this.drawAttackLane(
      planets[1],
      planets[2],
      palette.accent,
      0.00014,
      0.44,
      3,
      14,
    );
    this.drawAttackLane(
      planets[2],
      planets[0],
      palette.friendly,
      0.00016,
      0.71,
      4,
      20,
    );
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

  private drawAttackLane(
    from: BattlePlanet,
    to: BattlePlanet,
    tint: number,
    speed: number,
    phase: number,
    squadSize: number,
    sway: number,
  ): void {
    const impactPulse =
      0.55 + 0.45 * Math.sin(this.animationTime * 0.004 + phase * 8);
    const dx = to.x - from.x;
    const dy = to.y - from.y;
    const distance = Math.hypot(dx, dy) || 1;
    const normalX = -dy / distance;
    const normalY = dx / distance;
    const beamEndX = to.x - (dx / distance) * to.radius * 0.8;
    const beamEndY = to.y - (dy / distance) * to.radius * 0.8;

    this.battleScene.moveTo(from.x, from.y);
    this.battleScene.lineTo(beamEndX, beamEndY);
    this.battleScene.stroke({
      color: tint,
      width: 1.4,
      alpha: 0.12 + impactPulse * 0.1,
    });

    this.battleScene.circle(
      beamEndX,
      beamEndY,
      to.radius * (0.2 + impactPulse * 0.24),
    );
    this.battleScene.stroke({ color: tint, width: 1.4, alpha: 0.3 });

    for (let shipIndex = 0; shipIndex < squadSize; shipIndex += 1) {
      const progress =
        (this.animationTime * speed + phase + shipIndex / squadSize) % 1;
      const x =
        from.x +
        dx * progress +
        normalX * Math.sin(progress * 9 + phase * 7) * sway;
      const y =
        from.y +
        dy * progress +
        normalY * Math.sin(progress * 9 + phase * 7) * sway;
      this.drawShip(x, y, Math.atan2(dy, dx), tint, 0.9);
    }
  }

  private drawShip(
    x: number,
    y: number,
    angle: number,
    tint: number,
    alpha: number,
  ): void {
    const noseX = x + Math.cos(angle) * 9;
    const noseY = y + Math.sin(angle) * 9;
    const rearLeftX = x + Math.cos(angle + 2.55) * 6;
    const rearLeftY = y + Math.sin(angle + 2.55) * 6;
    const rearRightX = x + Math.cos(angle - 2.55) * 6;
    const rearRightY = y + Math.sin(angle - 2.55) * 6;

    this.battleScene.moveTo(noseX, noseY);
    this.battleScene.lineTo(rearLeftX, rearLeftY);
    this.battleScene.lineTo(rearRightX, rearRightY);
    this.battleScene.closePath();
    this.battleScene.fill({ color: tint, alpha });
  }

  private lobbyAccent(status: LobbyStatus, joined: boolean): number {
    if (joined) {
      return palette.friendly;
    }

    switch (status) {
      case "countdown":
        return palette.accent;
      case "playing":
        return palette.enemy;
      default:
        return palette.outline;
    }
  }

  private buildStatus(model: LobbyPanelModel): string {
    if (model.joinedLobbyId === null || model.lobbyStatus === null) {
      return "Pick a staging room. Countdown starts as soon as two captains lock in.";
    }

    const countdownLabel =
      model.lobbyStatus === "countdown" && model.countdownMs !== null
        ? ` · starts in ${Math.max(1, Math.ceil(model.countdownMs / 1000))}s`
        : "";

    return `Joined ${model.joinedLobbyId} · ${model.lobbyPlayers} players · ${model.lobbyStatus}${countdownLabel}`;
  }

  private describeLobby(lobby: LobbySummary, joined: boolean): string {
    const countdownLabel =
      lobby.status === "countdown" && lobby.countdownMs !== null
        ? ` · ${Math.max(1, Math.ceil(lobby.countdownMs / 1000))}s`
        : "";
    const joinedLabel = joined ? " · locked in" : "";
    return `${lobby.id.toUpperCase()}  |  ${lobby.players}/${lobby.maxPlayers} pilots  |  ${lobby.status}${countdownLabel}${joinedLabel}`;
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
