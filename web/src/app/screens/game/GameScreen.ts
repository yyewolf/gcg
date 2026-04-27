import type { Ticker } from "pixi.js";
import { Container } from "pixi.js";

import {
  GameClient,
  type ConnectionStatus,
  type GameClientEvent,
} from "../../game/GameClient";
import type {
  GameOverMessage,
  LobbyMessage,
  LobbyStatus,
  LobbySummary,
  PlanetSnapshot,
  Snapshot,
  WelcomeMessage,
  StateMessage,
} from "../../game/protocol";

import { GameBoard } from "./scene/GameBoard";
import { GameHud } from "./ui/GameHud";
import { DebugMenu } from "./ui/DebugMenu";
import { LobbyPanel } from "./ui/LobbyPanel";
import { GameResultPopup, type GameResult } from "../../popups/GameResultPopup";

const PERCENTAGE_STEP = 10;

export class GameScreen extends Container {
  private readonly resultPopup = new GameResultPopup(() => {
    void this.dismissResultPopup();
  });
  private readonly lobbyPanel = new LobbyPanel({
    onPlay: () => {
      this.client.play();
    },
  });
  private readonly board = new GameBoard({
    onAdjustSendPercentage: (delta) => this.adjustPercentage(delta),
    onClearSelection: () => this.clearSelection(),
    onPlanetActivate: (planetID, additive) =>
      this.handlePlanetActivate(planetID, additive),
    onPlanetBoxSelect: (planetIDs, additive) =>
      this.handlePlanetBoxSelect(planetIDs, additive),
  });
  private readonly client = new GameClient();
  private readonly hud = new GameHud();
  private readonly debugMenu = new DebugMenu({
    onTogglePathPredictions: (enabled) => {
      this.board.setShowDebugFleetTrails(enabled);
      this.showDebugPathPredictions = enabled;
      this.render();
    },
  });
  private unsubscribe: (() => void) | null = null;
  private paused = false;
  private connectionStatus: ConnectionStatus = "idle";
  private errorMessage: string | null = null;
  private mapName = "sector";
  private lobbyStatus: LobbyStatus | null = null;
  private joinedLobbyId: string | null = null;
  private lobbyPlayers = 0;
  private lobbyCountdownMS: number | null = null;
  private lobbies: LobbySummary[] = [];
  private playerID: number | null = null;
  private result: GameResult | null = null;
  private readonly selectedSourceIDs = new Set<number>();
  private sendPercentage = 50;
  private showDebugPathPredictions = false;
  private snapshot: Snapshot | null = null;
  private lastBoardSyncSnapshot: Snapshot | null = null;
  private lastBoardSyncPlayerID: number | null = null;
  private lastBoardSyncSelectionKey = "";
  private readonly onContextMenu = (event: MouseEvent) => {
    event.preventDefault();
  };
  private readonly onKeyDown = (event: KeyboardEvent) => {
    this.handleKeyDown(event);
  };

  constructor() {
    super();
    this.addChild(
      this.board,
      this.hud,
      this.lobbyPanel,
      this.debugMenu,
      this.resultPopup,
    );
  }

  public prepare(): void {
    this.render();
  }

  public async show(): Promise<void> {
    window.addEventListener("keydown", this.onKeyDown);
    window.addEventListener("contextmenu", this.onContextMenu);
    this.unsubscribe = this.client.subscribe((event) => {
      this.handleClientEvent(event);
    });
    this.client.connect();
  }

  public async hide(): Promise<void> {
    window.removeEventListener("keydown", this.onKeyDown);
    window.removeEventListener("contextmenu", this.onContextMenu);
    this.unsubscribe?.();
    this.unsubscribe = null;
    this.client.disconnect();
  }

  public async pause(): Promise<void> {
    this.paused = true;
  }

  public async resume(): Promise<void> {
    this.paused = false;
  }

  public reset(): void {
    this.errorMessage = null;
    this.result = null;
    this.selectedSourceIDs.clear();
  }

  public resize(width: number, height: number): void {
    this.board.resize(width, height);
    this.hud.resize(width, height);
    this.lobbyPanel.resize(width, height);
    this.debugMenu.resize();
    this.resultPopup.resize(width, height);
  }

  public update(time: Ticker): void {
    if (this.lobbyPanel.visible) {
      this.lobbyPanel.update(time.deltaMS);
    }

    if (!this.paused && this.snapshot !== null) {
      this.board.update(time.deltaMS);
    }
  }

  private handleClientEvent(event: GameClientEvent): void {
    switch (event.type) {
      case "connection":
        this.connectionStatus = event.status;
        if (event.status === "connecting") {
          this.errorMessage = null;
        }
        this.render();
        return;
      case "error":
        this.errorMessage = event.message;
        this.render();
        return;
      case "message":
        this.handleMessage(event.message);
        return;
    }
  }

  private handleMessage(
    message:
      | LobbyMessage
      | GameOverMessage
      | WelcomeMessage
      | StateMessage
      | { t: "error"; error: string },
  ): void {
    switch (message.t) {
      case "lobby":
        this.applyLobbyState(message);
        return;
      case "gameover":
        this.handleGameOver(message);
        return;
      case "welcome":
        this.playerID = message.playerId;
        this.mapName = message.map;
        this.applySnapshot(message.state);
        this.connectionStatus = "open";
        return;
      case "state":
        this.applySnapshot(message.state);
        return;
      case "error":
        this.errorMessage = message.error;
        this.render();
        return;
    }
  }

  private applyLobbyState(message: LobbyMessage): void {
    this.joinedLobbyId = message.joinedLobbyId;
    this.lobbyStatus = message.lobbyStatus;
    this.lobbyPlayers = message.lobbyPlayers;
    this.lobbyCountdownMS = message.countdownMs;
    this.lobbies = message.lobbies;
    if (message.lobbyStatus !== "playing") {
      this.snapshot = null;
      this.playerID = null;
      this.selectedSourceIDs.clear();
    }
    this.render();
  }

  private handleGameOver(message: GameOverMessage): void {
    const playerID = this.playerID;
    this.snapshot = null;
    this.playerID = null;
    this.selectedSourceIDs.clear();
    this.errorMessage = null;
    this.result =
      playerID !== null && message.winnerId === playerID ? "win" : "lose";
    this.resultPopup.setResult(this.result);
    void this.resultPopup.present();
    this.render();
  }

  private async dismissResultPopup(): Promise<void> {
    await this.resultPopup.dismiss();
    this.result = null;
    this.render();
  }

  private applySnapshot(snapshot: Snapshot): void {
    this.snapshot = snapshot;
    this.errorMessage = null;

    for (const planetID of Array.from(this.selectedSourceIDs)) {
      const selected = this.findPlanet(planetID);
      if (
        selected === null ||
        selected.owner !== this.playerID ||
        selected.ships < 1
      ) {
        this.selectedSourceIDs.delete(planetID);
      }
    }

    this.render();
  }

  private handlePlanetActivate(planetID: number, additive: boolean): void {
    if (this.snapshot === null || this.playerID === null) {
      return;
    }

    const planet = this.findPlanet(planetID);
    if (planet === null) {
      return;
    }

    if (additive) {
      if (planet.owner !== this.playerID || planet.ships < 1) {
        return;
      }

      if (this.selectedSourceIDs.has(planetID)) {
        this.selectedSourceIDs.delete(planetID);
      } else {
        this.selectedSourceIDs.add(planetID);
      }
      this.errorMessage = null;
      this.render();
      return;
    }

    if (this.selectedSourceIDs.has(planetID)) {
      if (this.selectedSourceIDs.size === 1) {
        this.clearSelection();
        return;
      }

      this.selectedSourceIDs.clear();
      this.selectedSourceIDs.add(planetID);
      this.errorMessage = null;
      this.render();
      return;
    }

    if (this.selectedSourceIDs.size > 0) {
      if (!this.sendSelectedFleets(planetID)) {
        return;
      }

      this.clearSelection(false);
      this.errorMessage = null;
      this.render();
      return;
    }

    if (planet.owner !== this.playerID || planet.ships < 1) {
      this.errorMessage = "Pick one of your planets to launch a fleet.";
      this.render();
      return;
    }

    this.selectedSourceIDs.clear();
    this.selectedSourceIDs.add(planetID);
    this.errorMessage = null;
    this.render();
  }

  private handlePlanetBoxSelect(planetIDs: number[], additive: boolean): void {
    if (this.snapshot === null || this.playerID === null) {
      return;
    }

    const nextSelection = additive
      ? new Set(this.selectedSourceIDs)
      : new Set<number>();

    for (const planetID of planetIDs) {
      const planet = this.findPlanet(planetID);
      if (
        planet !== null &&
        planet.owner === this.playerID &&
        planet.ships > 0
      ) {
        nextSelection.add(planetID);
      }
    }

    this.selectedSourceIDs.clear();
    for (const planetID of nextSelection) {
      this.selectedSourceIDs.add(planetID);
    }
    this.errorMessage = null;
    this.render();
  }

  private sendSelectedFleets(targetID: number): boolean {
    let sentAny = false;

    for (const sourceID of this.selectedSourceIDs) {
      if (sourceID === targetID) {
        continue;
      }
      if (!this.client.sendFleet(sourceID, targetID, this.sendPercentage)) {
        return false;
      }
      sentAny = true;
    }

    return sentAny;
  }

  private clearSelection(render = true): void {
    if (this.selectedSourceIDs.size === 0) {
      return;
    }

    this.selectedSourceIDs.clear();
    this.errorMessage = null;
    if (render) {
      this.render();
    }
  }

  private handleKeyDown(event: KeyboardEvent): void {
    if (event.key === "ArrowLeft") {
      this.adjustPercentage(-PERCENTAGE_STEP);
      return;
    }

    if (event.key === "ArrowRight") {
      this.adjustPercentage(PERCENTAGE_STEP);
      return;
    }

    if (event.key === "Escape") {
      this.clearSelection();
      return;
    }

    if (event.key.toLowerCase() === "r") {
      this.client.reconnect();
    }
  }

  private adjustPercentage(delta: number): void {
    this.sendPercentage = Math.max(
      10,
      Math.min(100, this.sendPercentage + delta),
    );
    this.render();
  }

  private render(): void {
    const inGame = this.snapshot !== null && this.playerID !== null;
    this.board.visible = inGame;
    this.hud.visible = inGame;
    this.lobbyPanel.visible = !inGame;
    this.resultPopup.visible = this.result !== null;
    const selectionKey = inGame
      ? Array.from(this.selectedSourceIDs)
          .sort((a, b) => a - b)
          .join(",")
      : "";
    if (
      this.lastBoardSyncSnapshot !== (inGame ? this.snapshot : null) ||
      this.lastBoardSyncPlayerID !== this.playerID ||
      this.lastBoardSyncSelectionKey !== selectionKey
    ) {
      this.board.sync(
        inGame ? this.snapshot : null,
        this.playerID,
        this.selectedSourceIDs,
      );
      this.lastBoardSyncSnapshot = inGame ? this.snapshot : null;
      this.lastBoardSyncPlayerID = this.playerID;
      this.lastBoardSyncSelectionKey = selectionKey;
    }

    if (inGame) {
      this.hud.render({
        sendPercentage: this.sendPercentage,
      });
    } else {
      this.lobbyPanel.render({
        connectionStatus: this.connectionStatus,
        errorMessage: this.errorMessage,
        joinedLobbyId: this.joinedLobbyId,
        lobbyStatus: this.lobbyStatus,
        lobbyPlayers: this.lobbyPlayers,
        countdownMs: this.lobbyCountdownMS,
        lobbies: this.lobbies,
      });
    }

    this.debugMenu.render({
      connectionStatus: this.connectionStatus,
      fleetCount: this.snapshot?.fleets.length ?? 0,
      inGame,
      joinedLobbyId: this.joinedLobbyId,
      lobbyPlayers: this.lobbyPlayers,
      lobbyStatus: this.lobbyStatus,
      mapName: this.mapName,
      planetCount: this.snapshot?.planets.length ?? 0,
      playerId: this.playerID,
      showPathPredictions: this.showDebugPathPredictions,
      tick: this.snapshot?.tick ?? null,
    });
  }

  private findPlanet(planetID: number): PlanetSnapshot | null {
    if (this.snapshot === null) {
      return null;
    }

    for (const planet of this.snapshot.planets) {
      if (planet.id === planetID) {
        return planet;
      }
    }

    return null;
  }
}
