import { Container, Graphics, Text } from "pixi.js";

import type { PlanetSnapshot } from "../../../game/protocol";

import { palette } from "../theme";

interface PlanetImpact {
  ageMS: number;
  durationMS: number;
  offsetX: number;
  offsetY: number;
  radius: number;
  driftX: number;
  driftY: number;
}

export interface PlanetPresentation {
  planet: PlanetSnapshot;
  selected: boolean;
  color: number;
  concealShips: boolean;
  neutral: boolean;
}

export class PlanetView extends Container {
  private readonly halo = new Graphics();
  private readonly body = new Graphics();
  private readonly impactLayer = new Graphics();
  private readonly shipLabel = new Text({
    anchor: 0.5,
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 18,
      fontWeight: "700",
    },
  });
  private readonly growthLabel = new Text({
    anchor: 0.5,
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 11,
      fontWeight: "600",
    },
  });
  private radius = 0;
  private selected = false;
  private pulse = 0;
  private readonly impacts: PlanetImpact[] = [];
  private lastColor: number | null = null;
  private lastNeutral: boolean | null = null;
  private lastShipText = "";

  constructor() {
    super();

    this.eventMode = "none";
    this.addChild(
      this.halo,
      this.body,
      this.impactLayer,
      this.shipLabel,
      this.growthLabel,
    );
  }

  public sync(presentation: PlanetPresentation): void {
    const nextRadius = presentation.planet.r;
    const nextShipText = presentation.concealShips
      ? "?"
      : String(presentation.planet.ships);

    if (this.radius !== nextRadius) {
      this.radius = nextRadius;
      this.lastColor = null;
      this.lastNeutral = null;
    }

    if (!this.selected && presentation.selected) {
      this.pulse = 0;
    }
    this.selected = presentation.selected;
    this.position.set(presentation.planet.x, presentation.planet.y);
    if (this.lastShipText !== nextShipText) {
      this.shipLabel.text = nextShipText;
      this.lastShipText = nextShipText;
    }
    this.shipLabel.visible = true;
    this.growthLabel.visible = false;
    if (
      this.lastColor !== presentation.color ||
      this.lastNeutral !== presentation.neutral
    ) {
      this.draw(presentation.color, presentation.neutral);
      this.lastColor = presentation.color;
      this.lastNeutral = presentation.neutral;
    }
  }

  public update(deltaMS: number): void {
    if (!this.selected) {
      this.halo.alpha = 0.18;
      this.halo.scale.set(1);
    } else {
      this.pulse = (this.pulse + deltaMS * 0.004) % (Math.PI * 2);
      const intensity = 0.55 + Math.sin(this.pulse) * 0.18;
      this.halo.alpha = intensity;
      this.halo.scale.set(1.04 + intensity * 0.08);
    }

    this.updateImpacts(deltaMS);
  }

  public addImpact(): void {
    const angle = Math.random() * Math.PI * 2;
    const offsetDistance = Math.random() * this.radius * 0.4;
    this.impacts.push({
      ageMS: 0,
      durationMS: 380,
      offsetX: Math.cos(angle) * offsetDistance,
      offsetY: Math.sin(angle) * offsetDistance,
      radius: 5 + Math.random() * 5,
      driftX: (Math.random() - 0.5) * 6,
      driftY: -8 - Math.random() * 10,
    });

    if (this.impacts.length > 24) {
      this.impacts.shift();
    }
  }

  private draw(color: number, neutral: boolean): void {
    this.halo.clear();
    this.halo.circle(0, 0, this.radius + 12);
    this.halo.fill({ color, alpha: 0.18 });

    this.body.clear();
    this.body.circle(0, 0, this.radius + 5);
    this.body.fill({ color: 0xffffff, alpha: 0.08 });
    this.body.circle(0, 0, this.radius + 2);
    this.body.stroke({ color, width: 3, alpha: 0.92 });
    this.body.circle(0, 0, this.radius);
    this.body.fill({ color, alpha: neutral ? 0.2 : 0.3 });
    this.body.circle(0, 0, this.radius * 0.55);
    this.body.fill({ color, alpha: 0.86 });
  }

  private updateImpacts(deltaMS: number): void {
    if (this.impacts.length === 0) {
      this.impactLayer.clear();
      return;
    }

    for (let index = this.impacts.length - 1; index >= 0; index -= 1) {
      const impact = this.impacts[index];
      impact.ageMS += deltaMS;
      if (impact.ageMS >= impact.durationMS) {
        this.impacts.splice(index, 1);
      }
    }

    this.impactLayer.clear();
    for (const impact of this.impacts) {
      const progress = impact.ageMS / impact.durationMS;
      const alpha = 1 - progress;
      const flameX = impact.offsetX + impact.driftX * progress;
      const flameY = impact.offsetY + impact.driftY * progress;
      const coreRadius = impact.radius * (1 - progress * 0.35);
      const glowRadius = impact.radius * (1.8 + progress * 1.4);

      this.impactLayer.circle(flameX, flameY, glowRadius);
      this.impactLayer.fill({ color: palette.enemy, alpha: alpha * 0.14 });

      this.impactLayer.circle(flameX, flameY - 1.5, coreRadius * 1.15);
      this.impactLayer.fill({ color: palette.warning, alpha: alpha * 0.78 });

      this.impactLayer.circle(flameX, flameY - 2.5, coreRadius * 0.62);
      this.impactLayer.fill({ color: palette.accent, alpha: alpha * 0.92 });

      this.impactLayer.circle(flameX, flameY - 3.2, coreRadius * 0.24);
      this.impactLayer.fill({ color: 0xffffff, alpha: alpha * 0.9 });

      for (let sparkIndex = 0; sparkIndex < 5; sparkIndex += 1) {
        const sparkAngle =
          -Math.PI * 0.92 + sparkIndex * 0.46 + progress * 0.35;
        const startX = flameX + Math.cos(sparkAngle) * coreRadius * 0.4;
        const startY = flameY + Math.sin(sparkAngle) * coreRadius * 0.3;
        const endX = flameX + Math.cos(sparkAngle) * glowRadius * 0.95;
        const endY = flameY + Math.sin(sparkAngle) * glowRadius * 1.45;
        this.impactLayer.moveTo(startX, startY);
        this.impactLayer.lineTo(endX, endY);
      }
      this.impactLayer.stroke({
        color: palette.warning,
        width: 1.4,
        alpha: alpha * 0.55,
      });
    }
  }
}
