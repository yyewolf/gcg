export const worldLayout = {
  hudHeight: 136,
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

export type OwnershipTone = "self" | "enemy" | "neutral";

export function ownershipColor(tone: OwnershipTone): number {
  switch (tone) {
    case "self":
      return palette.friendly;
    case "enemy":
      return palette.enemy;
    default:
      return palette.neutral;
  }
}
