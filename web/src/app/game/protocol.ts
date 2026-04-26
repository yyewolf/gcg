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

export interface Snapshot {
  tick: number;
  tickRate: number;
  width: number;
  height: number;
  planets: PlanetSnapshot[];
  fleets: FleetSnapshot[];
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
  state: Snapshot;
}

export interface ErrorMessage {
  t: "error";
  error: string;
}

export type ServerMessage = WelcomeMessage | StateMessage | ErrorMessage;

export interface SendFleetCommand {
  t: "send";
  src: number;
  dst: number;
  pct: number;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
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

function parseSnapshot(value: unknown): Snapshot | null {
  if (!isRecord(value)) {
    return null;
  }

  const { tick, tickRate, width, height, planets, fleets } = value;
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

  return {
    tick,
    tickRate,
    width,
    height,
    planets: parsedPlanets,
    fleets: parsedFleets,
  };
}

export function parseServerMessage(raw: string): ServerMessage | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    return null;
  }

  if (!isRecord(parsed) || typeof parsed.t !== "string") {
    return null;
  }

  switch (parsed.t) {
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

      return { t: "state", tick: parsed.tick, state };
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
