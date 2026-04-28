import { Container, Graphics } from "pixi.js";

import type { FleetSnapshot } from "../../../game/protocol";

const maxTrailPoints = 24;
const minTrailPointDistance = 4;
const shipLength = 9;
const shipHalfWidth = 4.8;
// How long (ms) to blend out a position correction after a server snapshot arrives.
const correctionDurationMS = 150;

interface TrailPoint {
  x: number;
  y: number;
}

export class FleetView extends Container {
  private readonly trail = new Graphics();
  private readonly body = new Graphics();
  private readonly heading = new Graphics();
  private readonly trailPoints: TrailPoint[] = [];
  private fleet: FleetSnapshot | null = null;
  private color = 0xffffff;
  private showDebugTrail: boolean;
  // Smooth correction: when a new server snapshot arrives, instead of
  // hard-teleporting, we record the gap between the predicted position and
  // the server position and decay it over correctionDurationMS.
  private correctionErrorX = 0;
  private correctionErrorY = 0;
  private correctionT = 1;

  constructor(showDebugTrail: boolean) {
    super();
    this.showDebugTrail = showDebugTrail;
    this.addChild(this.trail, this.heading, this.body);
  }

  public setShowDebugTrail(value: boolean): void {
    if (this.showDebugTrail === value) {
      return;
    }

    this.showDebugTrail = value;
    if (!value) {
      this.trailPoints.length = 0;
      this.trail.clear();
      this.heading.clear();
      return;
    }

    if (this.fleet !== null) {
      this.pushTrailPoint(this.fleet.x, this.fleet.y);
      this.drawTrail(this.fleet, this.color);
      this.drawHeading(this.fleet, this.color);
    }
  }

  public sync(fleet: FleetSnapshot, color: number): void {
    if (this.fleet !== null) {
      // Record the gap between where we're currently showing the fleet and
      // where the server says it is. predict() decays this error so there's
      // no hard visual jump on state arrival.
      this.correctionErrorX = this.position.x - fleet.x;
      this.correctionErrorY = this.position.y - fleet.y;
      this.correctionT = 0;
    } else {
      // First appearance: place the fleet directly at its server position
      // so it doesn't flash at the origin before predict() runs.
      this.position.set(fleet.x, fleet.y);
    }
    this.fleet = fleet;
    this.color = color;

    if (this.showDebugTrail) {
      this.pushTrailPoint(fleet.x, fleet.y);
      this.drawTrail(fleet, this.color);
      this.drawHeading(fleet, this.color);
    } else {
      this.trailPoints.length = 0;
      this.trail.clear();
      this.heading.clear();
    }
    this.drawShips(fleet, this.color);
  }

  public predict(elapsedSinceSyncMS: number, correctionDeltaMS: number): void {
    if (this.fleet === null) {
      return;
    }

    const deltaSeconds = elapsedSinceSyncMS / 1000;
    this.correctionT = Math.min(
      1,
      this.correctionT + correctionDeltaMS / correctionDurationMS,
    );
    // Position = server-authoritative forward prediction + decaying correction
    // that smooths out any gap between where we predicted the fleet and where
    // the server actually placed it.
    const correctionFactor = 1 - this.correctionT;
    const x =
      this.fleet.x +
      this.fleet.vx * deltaSeconds +
      this.correctionErrorX * correctionFactor;
    const y =
      this.fleet.y +
      this.fleet.vy * deltaSeconds +
      this.correctionErrorY * correctionFactor;
    this.position.set(x, y);

    if (this.showDebugTrail) {
      this.drawTrail(this.fleet, this.color);
      this.drawHeading(this.fleet, this.color);
    }
  }

  private pushTrailPoint(x: number, y: number): void {
    const lastPoint = this.trailPoints[this.trailPoints.length - 1];
    if (lastPoint !== undefined) {
      const dx = x - lastPoint.x;
      const dy = y - lastPoint.y;
      if (Math.hypot(dx, dy) < minTrailPointDistance) {
        lastPoint.x = x;
        lastPoint.y = y;
        return;
      }
    }

    this.trailPoints.push({ x, y });
    if (this.trailPoints.length > maxTrailPoints) {
      this.trailPoints.shift();
    }
  }

  private drawTrail(fleet: FleetSnapshot, color: number): void {
    this.trail.clear();
    if (this.trailPoints.length < 2) {
      return;
    }

    const firstPoint = this.trailPoints[0];
    this.trail.moveTo(firstPoint.x - fleet.x, firstPoint.y - fleet.y);
    for (let index = 1; index < this.trailPoints.length; index += 1) {
      const point = this.trailPoints[index];
      this.trail.lineTo(point.x - fleet.x, point.y - fleet.y);
    }

    this.trail.stroke({ color, width: 2, alpha: 0.34 });
  }

  private drawHeading(fleet: FleetSnapshot, color: number): void {
    this.heading.clear();

    const directionX = fleet.vx * 0.18;
    const directionY = fleet.vy * 0.18;
    if (Math.hypot(directionX, directionY) < 1) {
      return;
    }

    this.heading.moveTo(0, 0);
    this.heading.lineTo(directionX, directionY);
    this.heading.stroke({ color, width: 2, alpha: 0.5 });
  }

  private drawShips(fleet: FleetSnapshot, color: number): void {
    this.body.clear();
    this.body.rotation = this.resolveRotation(fleet);

    this.drawShipGlyph(0, 0);

    this.body.fill({ color, alpha: 0.94 });
  }

  private drawShipGlyph(x: number, y: number): void {
    this.body.moveTo(x + shipLength, y);
    this.body.lineTo(x - shipLength * 0.45, y - shipHalfWidth);
    this.body.lineTo(x - shipLength * 0.9, y);
    this.body.lineTo(x - shipLength * 0.45, y + shipHalfWidth);
    this.body.closePath();
  }

  private resolveRotation(fleet: FleetSnapshot): number {
    if (Math.hypot(fleet.vx, fleet.vy) < 1) {
      return 0;
    }

    return Math.atan2(fleet.vy, fleet.vx);
  }
}
