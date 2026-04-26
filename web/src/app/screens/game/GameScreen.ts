import type { Ticker } from "pixi.js";
import { Container } from "pixi.js";

import {
  GameClient,
  type ConnectionStatus,
  type GameClientEvent,
} from "../../game/GameClient";
import type {
  PlanetSnapshot,
  Snapshot,
  WelcomeMessage,
  StateMessage,
} from "../../game/protocol";

import { GameBoard } from "./scene/GameBoard";
import { GameHud } from "./ui/GameHud";

const PERCENTAGE_STEP = 10;

export class GameScreen extends Container {
  private readonly board = new GameBoard((planetID) =>
    this.handlePlanetTap(planetID),
  );
  private readonly client = new GameClient();
  private readonly hud = new GameHud({
    onDecreasePercentage: () => this.adjustPercentage(-PERCENTAGE_STEP),
    onIncreasePercentage: () => this.adjustPercentage(PERCENTAGE_STEP),
    onReconnect: () => this.client.reconnect(),
  });
  private unsubscribe: (() => void) | null = null;
  private paused = false;
  private connectionStatus: ConnectionStatus = "idle";
  private errorMessage: string | null = null;
  private mapName = "sector";
  private playerID: number | null = null;
  private selectedSourceID: number | null = null;
  private sendPercentage = 50;
  private snapshot: Snapshot | null = null;
  private readonly onKeyDown = (event: KeyboardEvent) => {
    this.handleKeyDown(event);
  };

  constructor() {
    super();
    this.addChild(this.board, this.hud);
  }

  public prepare(): void {
    this.render();
  }

  public async show(): Promise<void> {
    window.addEventListener("keydown", this.onKeyDown);
    this.unsubscribe = this.client.subscribe((event) => {
      this.handleClientEvent(event);
    });
    this.client.connect();
  }

  public async hide(): Promise<void> {
    window.removeEventListener("keydown", this.onKeyDown);
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
    this.selectedSourceID = null;
  }

  public resize(width: number, height: number): void {
    this.board.resize(width, height);
    this.hud.resize(width, height);
  }

  public update(time: Ticker): void {
    if (!this.paused) {
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
    message: WelcomeMessage | StateMessage | { t: "error"; error: string },
  ): void {
    switch (message.t) {
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

  private applySnapshot(snapshot: Snapshot): void {
    this.snapshot = snapshot;
    this.errorMessage = null;

    if (this.selectedSourceID !== null) {
      const selected = this.findPlanet(this.selectedSourceID);
      if (
        selected === null ||
        selected.owner !== this.playerID ||
        selected.ships < 1
      ) {
        this.selectedSourceID = null;
      }
    }

    this.render();
  }

  private handlePlanetTap(planetID: number): void {
    if (this.snapshot === null || this.playerID === null) {
      return;
    }

    const planet = this.findPlanet(planetID);
    if (planet === null) {
      return;
    }

    if (this.selectedSourceID === null) {
      if (planet.owner !== this.playerID) {
        this.errorMessage = "Pick one of your planets to launch a fleet.";
        this.render();
        return;
      }

      this.selectedSourceID = planetID;
      this.errorMessage = null;
      this.render();
      return;
    }

    if (planetID === this.selectedSourceID) {
      this.selectedSourceID = null;
      this.render();
      return;
    }

    if (
      !this.client.sendFleet(
        this.selectedSourceID,
        planetID,
        this.sendPercentage,
      )
    ) {
      return;
    }

    this.selectedSourceID = null;
    this.errorMessage = null;
    this.render();
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
      this.selectedSourceID = null;
      this.render();
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
    this.board.sync(this.snapshot, this.playerID, this.selectedSourceID);

    const selectedSource =
      this.selectedSourceID === null
        ? null
        : this.findPlanet(this.selectedSourceID);
    this.hud.render({
      connectionStatus: this.connectionStatus,
      errorMessage: this.errorMessage,
      fleetCount: this.snapshot?.fleets.length ?? 0,
      mapName: this.mapName,
      planetCount: this.snapshot?.planets.length ?? 0,
      playerId: this.playerID,
      selectedSourceText: this.describeSelection(selectedSource),
      sendPercentage: this.sendPercentage,
      tick: this.snapshot?.tick ?? null,
    });
  }

  private describeSelection(selectedSource: PlanetSnapshot | null): string {
    if (selectedSource === null) {
      return "No launch origin selected. Click one of your planets to arm a fleet.";
    }

    return `Source planet ${selectedSource.id} armed with ${selectedSource.ships} ships. Select a target to launch ${this.sendPercentage}%.`;
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
