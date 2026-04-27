import { Container, Graphics, Text } from "pixi.js";

import type { ConnectionStatus } from "../../../game/GameClient";

import { palette } from "../theme";

export interface DebugMenuModel {
  connectionStatus: ConnectionStatus;
  fleetCount: number;
  inGame: boolean;
  joinedLobbyId: string | null;
  lobbyPlayers: number;
  lobbyStatus: string | null;
  mapName: string;
  planetCount: number;
  playerId: number | null;
  showPathPredictions: boolean;
  tick: number | null;
}

interface DebugMenuCallbacks {
  onTogglePathPredictions: (enabled: boolean) => void;
}

export class DebugMenu extends Container {
  private readonly trigger = new Container();
  private readonly triggerBg = new Graphics();
  private readonly triggerLabel = new Text({
    text: "Debug",
    anchor: 0.5,
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 13,
      fontWeight: "700",
      letterSpacing: 1.4,
    },
  });
  private readonly panel = new Container();
  private readonly panelBg = new Graphics();
  private readonly panelTitle = new Text({
    text: "Debug Menu",
    anchor: { x: 0, y: 0 },
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 15,
      fontWeight: "700",
    },
  });
  private readonly info = new Text({
    anchor: { x: 0, y: 0 },
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 12,
      fontWeight: "600",
      lineHeight: 18,
    },
  });
  private readonly toggleRow = new Container();
  private readonly toggleBg = new Graphics();
  private readonly toggleLabel = new Text({
    text: "Show path predictions",
    anchor: { x: 0, y: 0.5 },
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 13,
      fontWeight: "600",
    },
  });
  private readonly toggleValue = new Text({
    anchor: { x: 1, y: 0.5 },
    style: {
      fill: palette.accent,
      fontFamily: "Trebuchet MS",
      fontSize: 12,
      fontWeight: "700",
      letterSpacing: 1.2,
    },
  });
  private model: DebugMenuModel = {
    connectionStatus: "idle",
    fleetCount: 0,
    inGame: false,
    joinedLobbyId: null,
    lobbyPlayers: 0,
    lobbyStatus: null,
    mapName: "sector",
    planetCount: 0,
    playerId: null,
    showPathPredictions: false,
    tick: null,
  };
  private open = false;

  constructor(private readonly callbacks: DebugMenuCallbacks) {
    super();

    this.trigger.eventMode = "static";
    this.trigger.cursor = "pointer";
    this.trigger.addChild(this.triggerBg, this.triggerLabel);
    this.trigger.on("pointertap", () => {
      this.setOpen(!this.open);
    });

    this.toggleRow.eventMode = "static";
    this.toggleRow.cursor = "pointer";
    this.toggleRow.addChild(this.toggleBg, this.toggleLabel, this.toggleValue);
    this.toggleRow.on("pointertap", () => {
      this.callbacks.onTogglePathPredictions(!this.model.showPathPredictions);
    });

    this.panel.addChild(
      this.panelBg,
      this.panelTitle,
      this.info,
      this.toggleRow,
    );

    this.addChild(this.trigger, this.panel);
    this.redraw();
    this.setOpen(false);
  }

  public resize(): void {
    this.position.set(20, 20);
  }

  public render(model: DebugMenuModel): void {
    this.model = model;
    this.info.text = this.buildInfo(model);
    this.toggleValue.text = model.showPathPredictions ? "ON" : "OFF";
    this.redraw();
  }

  private setOpen(value: boolean): void {
    this.open = value;
    this.panel.visible = value;
    this.redraw();
  }

  private redraw(): void {
    this.triggerBg.clear();
    this.triggerBg.roundRect(0, 0, 72, 30, 12);
    this.triggerBg.fill({ color: palette.panel, alpha: 0.88 });
    this.triggerBg.roundRect(0, 0, 72, 30, 12);
    this.triggerBg.stroke({
      color: this.open ? palette.accent : palette.outline,
      width: 1.5,
      alpha: 0.95,
    });
    this.triggerLabel.position.set(36, 15);

    this.panel.position.set(0, 40);

    this.panelBg.clear();
    this.panelBg.roundRect(0, 0, 256, 144, 18);
    this.panelBg.fill({ color: palette.panel, alpha: 0.92 });
    this.panelBg.roundRect(0, 0, 256, 144, 18);
    this.panelBg.stroke({ color: palette.outline, width: 1.5, alpha: 1 });

    this.panelTitle.position.set(16, 14);
    this.info.position.set(16, 40);

    this.toggleBg.clear();
    this.toggleBg.roundRect(0, 0, 224, 40, 12);
    this.toggleBg.fill({ color: palette.surfaceAlt, alpha: 0.58 });
    this.toggleBg.roundRect(0, 0, 224, 40, 12);
    this.toggleBg.stroke({
      color: this.model.showPathPredictions
        ? palette.friendly
        : palette.outline,
      width: 1.5,
      alpha: 0.92,
    });
    this.toggleRow.position.set(16, 92);
    this.toggleLabel.position.set(14, 20);
    this.toggleValue.position.set(210, 20);
  }

  private buildInfo(model: DebugMenuModel): string {
    const connection = this.formatConnection(model.connectionStatus);
    if (model.inGame) {
      const playerLabel =
        model.playerId === null ? "spectator" : `player ${model.playerId}`;
      const tickLabel = model.tick === null ? "--" : `${model.tick}`;
      return `Mode: match\nConnection: ${connection}\nMap: ${model.mapName}\nPlayer: ${playerLabel}\nTick: ${tickLabel} · ${model.planetCount} planets · ${model.fleetCount} fleets`;
    }

    const lobbyLabel =
      model.joinedLobbyId === null || model.lobbyStatus === null
        ? "queue idle"
        : `${model.joinedLobbyId} · ${model.lobbyPlayers} players · ${model.lobbyStatus}`;
    return `Mode: lobby\nConnection: ${connection}\nLobby: ${lobbyLabel}`;
  }

  private formatConnection(status: ConnectionStatus): string {
    switch (status) {
      case "open":
        return "online";
      case "connecting":
        return "connecting";
      case "closed":
        return "offline";
      default:
        return "idle";
    }
  }
}
