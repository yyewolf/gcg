import { Container, Graphics, Text } from "pixi.js";

import type { ConnectionStatus } from "../../../game/GameClient";
import type { LobbyStatus, LobbySummary } from "../../../game/protocol";

import { palette } from "../theme";

import { HudButton } from "./HudButton";

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
  private static readonly listSidePadding = 32;
  private static readonly listRowGap = 58;
  private static readonly listTopOffset = 178;
  private static readonly listButtonWidth = 108;
  private static readonly listButtonGap = 20;
  private static readonly listRightPadding = 28;
  private static readonly statusWidthPadding = 52;
  private static readonly errorWidthPadding = 52;

  private readonly backdrop = new Graphics();
  private readonly panel = new Graphics();
  private readonly title = new Text({
    anchor: 0.5,
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 34,
      fontWeight: "700",
    },
  });
  private readonly status = new Text({
    anchor: 0.5,
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 16,
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
  private readonly reconnectButton: HudButton;
  private readonly entryLabels = new Map<string, Text>();
  private readonly entryButtons = new Map<string, HudButton>();
  private panelX = 0;
  private panelY = 0;
  private panelWidth = 0;
  private panelHeight = 0;

  constructor(private readonly callbacks: LobbyPanelCallbacks) {
    super();

    this.title.text = "Lobby";
    this.reconnectButton = new HudButton({
      label: "Reconnect",
      width: 130,
      height: 42,
      tint: 0x163248,
      onPress: callbacks.onReconnect,
    });

    this.addChild(
      this.backdrop,
      this.panel,
      this.title,
      this.status,
      this.error,
      this.reconnectButton,
    );
  }

  public resize(width: number, height: number): void {
    this.backdrop.clear();
    this.backdrop.rect(0, 0, width, height);
    this.backdrop.fill({ color: palette.background, alpha: 1 });

    this.panelWidth = Math.min(width - 80, 760);
    this.panelHeight = Math.min(height - 100, 560);
    this.panelX = (width - this.panelWidth) * 0.5;
    this.panelY = (height - this.panelHeight) * 0.5;

    this.panel.clear();
    this.panel.roundRect(
      this.panelX,
      this.panelY,
      this.panelWidth,
      this.panelHeight,
      28,
    );
    this.panel.fill({ color: palette.panel, alpha: 0.96 });
    this.panel.roundRect(
      this.panelX,
      this.panelY,
      this.panelWidth,
      this.panelHeight,
      28,
    );
    this.panel.stroke({ color: palette.outline, width: 2, alpha: 1 });

    this.title.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + 54,
    );
    this.status.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + 96,
    );
    this.error.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + 126,
    );
    this.reconnectButton.position.set(
      this.panelX + this.panelWidth * 0.5,
      this.panelY + this.panelHeight - 42,
    );

    this.status.style.wordWrap = true;
    this.status.style.wordWrapWidth =
      this.panelWidth - LobbyPanel.statusWidthPadding;
    this.error.style.wordWrap = true;
    this.error.style.wordWrapWidth =
      this.panelWidth - LobbyPanel.errorWidthPadding;
  }

  public render(model: LobbyPanelModel): void {
    this.status.text = this.buildStatus(model);
    this.status.style.fill = this.statusColor(model.connectionStatus);
    this.error.text = model.errorMessage ?? "";
    this.error.visible = model.errorMessage !== null;
    this.reconnectButton.setDisabled(model.connectionStatus === "connecting");
    this.syncEntries(model);
  }

  private syncEntries(model: LobbyPanelModel): void {
    const activeIDs = new Set<string>();
    const startY = this.panelY + LobbyPanel.listTopOffset;
    const labelX = this.panelX + LobbyPanel.listSidePadding;
    const buttonX =
      this.panelX +
      this.panelWidth -
      LobbyPanel.listRightPadding -
      LobbyPanel.listButtonWidth * 0.5;
    const labelWidth = Math.max(
      120,
      this.panelWidth -
        LobbyPanel.listSidePadding -
        LobbyPanel.listButtonWidth -
        LobbyPanel.listButtonGap -
        LobbyPanel.listRightPadding,
    );

    model.lobbies.forEach((lobby, index) => {
      activeIDs.add(lobby.id);
      let label = this.entryLabels.get(lobby.id);
      if (label === undefined) {
        label = new Text({
          anchor: { x: 0, y: 0.5 },
          style: {
            fill: palette.text,
            fontFamily: "Trebuchet MS",
            fontSize: 18,
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
          width: LobbyPanel.listButtonWidth,
          height: 40,
          onPress: () => this.callbacks.onJoinLobby(lobby.id),
        });
        this.entryButtons.set(lobby.id, button);
        this.addChild(button);
      }

      label.text = this.describeLobby(lobby, model.joinedLobbyId === lobby.id);
      label.style.wordWrapWidth = labelWidth;
      label.position.set(labelX, startY + index * LobbyPanel.listRowGap);
      button.position.set(buttonX, startY + index * LobbyPanel.listRowGap);
      button.setDisabled(
        lobby.status === "playing" || model.joinedLobbyId === lobby.id,
      );
    });

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

  private buildStatus(model: LobbyPanelModel): string {
    if (model.joinedLobbyId === null || model.lobbyStatus === null) {
      return "Choose a lobby to join. A countdown starts as soon as two players are inside.";
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
    const joinedLabel = joined ? " · joined" : "";
    return `${lobby.id} · ${lobby.players}/${lobby.maxPlayers} · ${lobby.status}${countdownLabel}${joinedLabel}`;
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
