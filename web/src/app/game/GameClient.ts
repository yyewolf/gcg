import { decode as cborDecode, encode as cborEncode } from "cbor-x";
import {
  type JoinLobbyCommand,
  parseServerMessage,
  type PlayCommand,
  type SendFleetCommand,
  type ServerMessage,
} from "./protocol";

export type ConnectionStatus = "idle" | "connecting" | "open" | "closed";

export type GameClientEvent =
  | { type: "connection"; status: ConnectionStatus }
  | { type: "message"; message: ServerMessage }
  | { type: "error"; message: string };

type Listener = (event: GameClientEvent) => void;

const RECONNECT_DELAY_MS = 1500;

export class GameClient {
  private readonly listeners = new Set<Listener>();
  private reconnectTimer: number | null = null;
  private socket: WebSocket | null = null;
  private manuallyClosed = false;

  public subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  public connect(): void {
    if (
      this.socket !== null &&
      (this.socket.readyState === WebSocket.OPEN ||
        this.socket.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }

    this.clearReconnectTimer();
    this.manuallyClosed = false;
    this.emit({ type: "connection", status: "connecting" });

    const socket = new WebSocket(this.resolveURL());
    this.socket = socket;

    socket.addEventListener("open", () => {
      this.emit({ type: "connection", status: "open" });
    });

    socket.binaryType = "arraybuffer";

    socket.addEventListener("message", (event: MessageEvent) => {
      this.handleMessage(event);
    });

    socket.addEventListener("close", () => {
      if (this.socket === socket) {
        this.socket = null;
      }
      this.emit({ type: "connection", status: "closed" });
      if (!this.manuallyClosed) {
        this.scheduleReconnect();
      }
    });

    socket.addEventListener("error", () => {
      this.emit({ type: "error", message: "WebSocket connection error." });
    });
  }

  public disconnect(): void {
    this.manuallyClosed = true;
    this.clearReconnectTimer();

    if (this.socket === null) {
      this.emit({ type: "connection", status: "closed" });
      return;
    }

    this.socket.close();
    this.socket = null;
  }

  public reconnect(): void {
    this.disconnect();
    this.manuallyClosed = false;
    this.connect();
  }

  public sendFleet(src: number, dst: number, pct: number): boolean {
    if (this.socket === null || this.socket.readyState !== WebSocket.OPEN) {
      this.emit({
        type: "error",
        message: "Connection is not ready yet.",
      });
      return false;
    }

    const command: SendFleetCommand = { t: "send", src, dst, pct };
    this.socket.send(cborEncode(command));
    return true;
  }

  public joinLobby(lobby: string): boolean {
    if (this.socket === null || this.socket.readyState !== WebSocket.OPEN) {
      this.emit({
        type: "error",
        message: "Connection is not ready yet.",
      });
      return false;
    }

    const command: JoinLobbyCommand = { t: "join", lobby };
    this.socket.send(cborEncode(command));
    return true;
  }

  public play(): boolean {
    if (this.socket === null || this.socket.readyState !== WebSocket.OPEN) {
      this.emit({
        type: "error",
        message: "Connection is not ready yet.",
      });
      return false;
    }

    const command: PlayCommand = { t: "play" };
    this.socket.send(cborEncode(command));
    return true;
  }

  private handleMessage(event: MessageEvent): void {
    if (!(event.data instanceof ArrayBuffer)) {
      this.emit({
        type: "error",
        message: "Received a non-binary server payload.",
      });
      return;
    }

    let decoded: unknown;
    try {
      decoded = cborDecode(new Uint8Array(event.data)) as unknown;
    } catch {
      this.emit({
        type: "error",
        message: "Failed to decode CBOR server payload.",
      });
      return;
    }

    const message = parseServerMessage(decoded);
    if (message === null) {
      this.emit({
        type: "error",
        message: "Received an invalid server message.",
      });
      return;
    }

    this.emit({ type: "message", message });
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer !== null) {
      return;
    }

    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.manuallyClosed) {
        this.connect();
      }
    }, RECONNECT_DELAY_MS);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer === null) {
      return;
    }

    window.clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }

  private resolveURL(): string {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/ws`;
  }

  private emit(event: GameClientEvent): void {
    for (const listener of this.listeners) {
      listener(event);
    }
  }
}
