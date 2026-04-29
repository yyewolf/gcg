export interface PlanetSnapshot {
  id: number;
  x: number;
  y: number;
  r: number;
  owner: number;
  ships: number;
  growth: number;
}

export interface FleetSnapshot {
  id: number;
  owner: number;
  src: number;
  dst: number;
  ships: number;
  x: number;
  y: number;
  vx: number;
  vy: number;
}

export interface PlayerColor {
  playerId: number;
  color: number;
}

export interface PlanetStateUpdate {
  id: number;
  owner: number;
  ships: number;
}

export interface Snapshot {
  tick: number;
  tickRate: number;
  width: number;
  height: number;
  planets: PlanetSnapshot[];
  fleets: FleetSnapshot[];
  playerColors: PlayerColor[];
}

export type LobbyStatus = "waiting" | "countdown" | "playing";

export interface LobbySummary {
  id: string;
  players: number;
  maxPlayers: number;
  status: LobbyStatus;
  countdownMs: number | null;
}

export interface LobbyMessage {
  t: "lobby";
  joinedLobbyId: string | null;
  lobbyStatus: LobbyStatus | null;
  lobbyPlayers: number;
  countdownMs: number | null;
  lobbies: LobbySummary[];
}

export interface WelcomeMessage {
  t: "welcome";
  playerId: number;
  tick: number;
  tickRate: number;
  map: string;
  state: Snapshot;
}

export interface StateMessage {
  t: "state";
  tick: number;
  tickRate: number;
  planets: PlanetStateUpdate[];
  fleets: FleetSnapshot[];
}

export interface GameOverMessage {
  t: "gameover";
  winnerId: number;
}

export interface ErrorMessage {
  t: "error";
  error: string;
}

export type ServerMessage =
  | LobbyMessage
  | WelcomeMessage
  | StateMessage
  | GameOverMessage
  | ErrorMessage;

export interface JoinLobbyCommand {
  t: "join";
  lobby: string;
}

export interface PlayCommand {
  t: "play";
}

export interface SendFleetCommand {
  t: "send";
  src: number;
  dst: number;
  pct: number;
}

export interface SendManyCommand {
  t: "sendmany";
  srcs: number[];
  dst: number;
  pct: number;
}

// BINARY STATE FRAME
// Magic byte that identifies a binary state frame (not valid as a CBOR map header).
export const BINARY_STATE_MAGIC = 0x01;

/**
 * Decode a binary state frame written by encodeBinaryState on the server.
 * Layout (little-endian):
 *   [0]       uint8   magic
 *   [1..8]    int64   tick
 *   [9]       uint8   tickRate
 *   [10..11]  uint16  nPlanets
 *   per planet (7 bytes): id(u16) owner(u8) ships(i32)
 *   [n..n+1]  uint16  nFleets
 *   per fleet (27 bytes): id(u32) owner(u8) src(u16) dst(u16) ships(u16)
 *                         x(f32) y(f32) vx(f32) vy(f32)
 */
export function parseBinaryStateFrame(data: ArrayBuffer): StateMessage | null {
  const view = new DataView(data);
  if (view.byteLength < 12) return null;

  const tick = Number(view.getBigInt64(1, true));
  const tickRate = view.getUint8(9);
  const nPlanets = view.getUint16(10, true);

  let off = 12;
  if (view.byteLength < off + nPlanets * 7) return null;

  const planets: PlanetStateUpdate[] = [];
  for (let i = 0; i < nPlanets; i++) {
    const id = view.getUint16(off, true);
    const owner = view.getUint8(off + 2);
    const ships = view.getInt32(off + 3, true);
    off += 7;
    planets.push({ id, owner, ships });
  }

  if (view.byteLength < off + 2) return null;
  const nFleets = view.getUint16(off, true);
  off += 2;

  if (view.byteLength < off + nFleets * 27) return null;

  const fleets: FleetSnapshot[] = [];
  for (let i = 0; i < nFleets; i++) {
    const id = view.getUint32(off, true);
    const owner = view.getUint8(off + 4);
    const src = view.getUint16(off + 5, true);
    const dst = view.getUint16(off + 7, true);
    const ships = view.getUint16(off + 9, true);
    const x = view.getFloat32(off + 11, true);
    const y = view.getFloat32(off + 15, true);
    const vx = view.getFloat32(off + 19, true);
    const vy = view.getFloat32(off + 23, true);
    off += 27;
    fleets.push({ id, owner, src, dst, ships, x, y, vx, vy });
  }

  return { t: "state", tick, tickRate, planets, fleets };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function isLobbyStatus(value: unknown): value is LobbyStatus {
  return value === "waiting" || value === "countdown" || value === "playing";
}

function parsePlanet(value: unknown): PlanetSnapshot | null {
  if (!isRecord(value)) {
    return null;
  }

  const { id, x, y, r, owner, ships, growth } = value;
  if (
    !isNumber(id) ||
    !isNumber(x) ||
    !isNumber(y) ||
    !isNumber(r) ||
    !isNumber(owner) ||
    !isNumber(ships) ||
    !isNumber(growth)
  ) {
    return null;
  }

  return { id, x, y, r, owner, ships, growth };
}

function parseFleet(value: unknown): FleetSnapshot | null {
  if (!isRecord(value)) {
    return null;
  }

  const { id, owner, src, dst, ships, x, y, vx, vy } = value;
  if (
    !isNumber(id) ||
    !isNumber(owner) ||
    !isNumber(src) ||
    !isNumber(dst) ||
    !isNumber(ships) ||
    !isNumber(x) ||
    !isNumber(y) ||
    !isNumber(vx) ||
    !isNumber(vy)
  ) {
    return null;
  }

  return { id, owner, src, dst, ships, x, y, vx, vy };
}

function parseLobbySummary(value: unknown): LobbySummary | null {
  if (!isRecord(value)) {
    return null;
  }

  const { id, players, maxPlayers, status, countdownMs } = value;
  if (
    typeof id !== "string" ||
    !isNumber(players) ||
    !isNumber(maxPlayers) ||
    !isLobbyStatus(status)
  ) {
    return null;
  }

  if (
    countdownMs !== undefined &&
    countdownMs !== null &&
    !isNumber(countdownMs)
  ) {
    return null;
  }

  return {
    id,
    players,
    maxPlayers,
    status,
    countdownMs: countdownMs ?? null,
  };
}

function parsePlayerColor(value: unknown): PlayerColor | null {
  if (!isRecord(value)) {
    return null;
  }

  const { playerId, color } = value;
  if (!isNumber(playerId) || !isNumber(color)) {
    return null;
  }

  return { playerId, color };
}

function parseSnapshot(value: unknown): Snapshot | null {
  if (!isRecord(value)) {
    return null;
  }

  const { tick, tickRate, width, height, planets, fleets, playerColors } =
    value;
  if (
    !isNumber(tick) ||
    !isNumber(tickRate) ||
    !isNumber(width) ||
    !isNumber(height)
  ) {
    return null;
  }

  if (!Array.isArray(planets) || !Array.isArray(fleets)) {
    return null;
  }
  if (playerColors !== undefined && !Array.isArray(playerColors)) {
    return null;
  }

  const parsedPlanets: PlanetSnapshot[] = [];
  for (const planet of planets) {
    const parsedPlanet = parsePlanet(planet);
    if (parsedPlanet === null) {
      return null;
    }
    parsedPlanets.push(parsedPlanet);
  }

  const parsedFleets: FleetSnapshot[] = [];
  for (const fleet of fleets) {
    const parsedFleet = parseFleet(fleet);
    if (parsedFleet === null) {
      return null;
    }
    parsedFleets.push(parsedFleet);
  }

  const parsedPlayerColors: PlayerColor[] = [];
  for (const playerColor of playerColors ?? []) {
    const parsedPlayerColor = parsePlayerColor(playerColor);
    if (parsedPlayerColor === null) {
      return null;
    }
    parsedPlayerColors.push(parsedPlayerColor);
  }

  return {
    tick,
    tickRate,
    width,
    height,
    planets: parsedPlanets,
    fleets: parsedFleets,
    playerColors: parsedPlayerColors,
  };
}

export function parseServerMessage(parsed: unknown): ServerMessage | null {
  if (!isRecord(parsed) || typeof parsed.t !== "string") {
    return null;
  }

  switch (parsed.t) {
    case "lobby": {
      if (!Array.isArray(parsed.lobbies)) {
        return null;
      }

      if (
        parsed.joinedLobbyId !== undefined &&
        parsed.joinedLobbyId !== null &&
        typeof parsed.joinedLobbyId !== "string"
      ) {
        return null;
      }
      if (
        parsed.lobbyStatus !== undefined &&
        parsed.lobbyStatus !== null &&
        !isLobbyStatus(parsed.lobbyStatus)
      ) {
        return null;
      }
      if (
        parsed.lobbyPlayers !== undefined &&
        parsed.lobbyPlayers !== null &&
        !isNumber(parsed.lobbyPlayers)
      ) {
        return null;
      }
      if (
        parsed.countdownMs !== undefined &&
        parsed.countdownMs !== null &&
        !isNumber(parsed.countdownMs)
      ) {
        return null;
      }

      const lobbies: LobbySummary[] = [];
      for (const lobby of parsed.lobbies) {
        const parsedLobby = parseLobbySummary(lobby);
        if (parsedLobby === null) {
          return null;
        }
        lobbies.push(parsedLobby);
      }

      return {
        t: "lobby",
        joinedLobbyId: parsed.joinedLobbyId ?? null,
        lobbyStatus: parsed.lobbyStatus ?? null,
        lobbyPlayers: parsed.lobbyPlayers ?? 0,
        countdownMs: parsed.countdownMs ?? null,
        lobbies,
      };
    }
    case "welcome": {
      if (
        !isNumber(parsed.playerId) ||
        !isNumber(parsed.tick) ||
        !isNumber(parsed.tickRate) ||
        typeof parsed.map !== "string"
      ) {
        return null;
      }

      const state = parseSnapshot(parsed.state);
      if (state === null) {
        return null;
      }

      return {
        t: "welcome",
        playerId: parsed.playerId,
        tick: parsed.tick,
        tickRate: parsed.tickRate,
        map: parsed.map,
        state,
      };
    }
    case "state": {
      if (!isNumber(parsed.tick)) {
        return null;
      }

      const state = parseSnapshot(parsed.state);
      if (state === null) {
        return null;
      }

      return {
        t: "state",
        tick: parsed.tick,
        tickRate: state.tickRate,
        planets: state.planets.map((planet) => ({
          id: planet.id,
          owner: planet.owner,
          ships: planet.ships,
        })),
        fleets: state.fleets,
      };
    }
    case "gameover": {
      if (!isNumber(parsed.winnerId)) {
        return null;
      }

      return { t: "gameover", winnerId: parsed.winnerId };
    }
    case "error": {
      if (typeof parsed.error !== "string") {
        return null;
      }

      return { t: "error", error: parsed.error };
    }
    default:
      return null;
  }
}
