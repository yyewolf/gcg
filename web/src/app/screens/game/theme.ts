export const worldLayout = {
  hudHeight: 72,
  padding: 20,
  worldWidth: 1400,
  worldHeight: 840,
};

export const palette = {
  background: 0x030712,
  surface: 0x071522,
  surfaceAlt: 0x0c2437,
  panel: 0x07111c,
  outline: 0x18354b,
  text: 0xe8f4ff,
  mutedText: 0x8aa3b9,
  friendly: 0x63f0ff,
  enemy: 0xff728c,
  neutral: 0x96a8bd,
  accent: 0xffd166,
  warning: 0xffa94d,
};

const playerColorPalette = [
  0x63f0ff, 0xff728c, 0x8bff6a, 0xffd166, 0xc792ff, 0xff9f5c, 0x5eead4,
  0xf472b6, 0x60a5fa, 0xfacc15, 0xfb7185, 0x34d399,
];

export function defaultPlayerColor(playerID: number): number {
  if (playerID <= 0) {
    return palette.neutral;
  }

  return playerColorPalette[(playerID - 1) % playerColorPalette.length];
}

export function resolveOwnershipColor(
  owner: number,
  playerColors: ReadonlyMap<number, number>,
): number {
  if (owner === 0) {
    return palette.neutral;
  }

  return playerColors.get(owner) ?? defaultPlayerColor(owner);
}
