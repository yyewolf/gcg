import { Container, Graphics, Text } from "pixi.js";

import type { ConnectionStatus } from "../../../game/GameClient";

import { palette } from "../theme";

import { HudButton } from "./HudButton";

export interface HudViewModel {
  connectionStatus: ConnectionStatus;
  errorMessage: string | null;
  fleetCount: number;
  mapName: string;
  planetCount: number;
  playerId: number | null;
  selectedSourceText: string;
  sendPercentage: number;
  tick: number | null;
}

interface GameHudCallbacks {
  onDecreasePercentage: () => void;
  onIncreasePercentage: () => void;
  onReconnect: () => void;
}

export class GameHud extends Container {
  private readonly chrome = new Graphics();
  private readonly title = new Text({
    anchor: { x: 0, y: 0.5 },
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 26,
      fontWeight: "700",
    },
  });
  private readonly status = new Text({
    anchor: { x: 0, y: 0.5 },
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 14,
      fontWeight: "600",
    },
  });
  private readonly selection = new Text({
    anchor: { x: 0, y: 0.5 },
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 14,
      fontWeight: "600",
    },
  });
  private readonly help = new Text({
    anchor: { x: 0.5, y: 0.5 },
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 13,
      fontWeight: "500",
    },
  });
  private readonly percentage = new Text({
    anchor: 0.5,
    style: {
      fill: palette.accent,
      fontFamily: "Trebuchet MS",
      fontSize: 22,
      fontWeight: "700",
    },
  });
  private readonly decreaseButton: HudButton;
  private readonly increaseButton: HudButton;
  private readonly reconnectButton: HudButton;

  constructor(callbacks: GameHudCallbacks) {
    super();

    this.decreaseButton = new HudButton({
      label: "-10",
      width: 62,
      height: 40,
      onPress: callbacks.onDecreasePercentage,
    });
    this.increaseButton = new HudButton({
      label: "+10",
      width: 62,
      height: 40,
      onPress: callbacks.onIncreasePercentage,
    });
    this.reconnectButton = new HudButton({
      label: "Reconnect",
      width: 120,
      height: 40,
      tint: 0x163248,
      onPress: callbacks.onReconnect,
    });

    this.help.text =
      "Click one of your planets, then click any target. Arrow keys adjust launch %. R retries the socket.";

    this.addChild(
      this.chrome,
      this.title,
      this.status,
      this.selection,
      this.help,
      this.percentage,
      this.decreaseButton,
      this.increaseButton,
      this.reconnectButton,
    );
  }

  public resize(width: number, height: number): void {
    this.chrome.clear();
    this.chrome.roundRect(18, 18, width - 36, 96, 24);
    this.chrome.fill({ color: palette.panel, alpha: 0.92 });
    this.chrome.roundRect(18, 18, width - 36, 96, 24);
    this.chrome.stroke({ color: palette.outline, width: 2, alpha: 1 });
    this.chrome.roundRect(18, height - 58, width - 36, 40, 20);
    this.chrome.fill({ color: palette.panel, alpha: 0.74 });

    this.title.position.set(40, 44);
    this.status.position.set(40, 74);
    this.selection.position.set(40, 100);
    this.decreaseButton.position.set(width - 218, 66);
    this.percentage.position.set(width - 146, 66);
    this.increaseButton.position.set(width - 74, 66);
    this.reconnectButton.position.set(width - 136, 100);
    this.help.position.set(width * 0.5, height - 38);
  }

  public render(model: HudViewModel): void {
    const statusLabel = this.buildStatusLabel(model);
    this.title.text = `GCG / ${model.mapName}`;
    this.status.text = statusLabel;
    this.status.style.fill = this.statusColor(model.connectionStatus);
    this.selection.text = model.errorMessage ?? model.selectedSourceText;
    this.selection.style.fill =
      model.errorMessage === null ? palette.text : palette.warning;
    this.percentage.text = `${model.sendPercentage}%`;
    this.reconnectButton.setDisabled(model.connectionStatus === "connecting");
  }

  private buildStatusLabel(model: HudViewModel): string {
    const playerLabel =
      model.playerId === null ? "spectator" : `player ${model.playerId}`;
    const tickLabel = model.tick === null ? "tick --" : `tick ${model.tick}`;
    return `${this.formatConnection(model.connectionStatus)} · ${playerLabel} · ${tickLabel} · ${model.planetCount} planets · ${model.fleetCount} fleets`;
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
}
