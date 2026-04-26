import { Container, Graphics, Text } from "pixi.js";

import type { FleetSnapshot } from "../../../game/protocol";

import { palette } from "../theme";

const maxTrailPoints = 24;
const minTrailPointDistance = 4;
const shipLength = 4.5;
const shipHalfWidth = 2.4;
const formationDepthGap = 6;
const formationWidthGap = 6;

interface TrailPoint {
  x: number;
  y: number;
}

export class FleetView extends Container {
  private readonly trail = new Graphics();
  private readonly body = new Graphics();
  private readonly heading = new Graphics();
  private readonly shipsLabel = new Text({
    anchor: 0.5,
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 11,
      fontWeight: "700",
    },
  });
  private readonly trailPoints: TrailPoint[] = [];

  constructor(private readonly showDebugTrail: boolean) {
    super();
    this.addChild(this.trail, this.heading, this.body, this.shipsLabel);
  }

  public sync(fleet: FleetSnapshot, isFriendly: boolean): void {
    const color = isFriendly ? palette.friendly : palette.enemy;
    this.position.set(fleet.x, fleet.y);
    if (fleet.ships > 1) {
      this.shipsLabel.text = String(fleet.ships);
      this.shipsLabel.y = -12;
      this.shipsLabel.visible = true;
    } else {
      this.shipsLabel.visible = false;
    }

    if (this.showDebugTrail) {
      this.pushTrailPoint(fleet.x, fleet.y);
      this.drawTrail(fleet, color);
      this.drawHeading(fleet, color);
    } else {
      this.trailPoints.length = 0;
      this.trail.clear();
      this.heading.clear();
    }
    this.drawShips(fleet, color);
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

    const shipCount = Math.max(1, Math.floor(fleet.ships));
    let remaining = shipCount;
    let rowSize = 1;
    let rowIndex = 0;

    while (remaining > 0) {
      const shipsInRow = Math.min(remaining, rowSize);
      const localX = -rowIndex * formationDepthGap;
      const rowWidth = (shipsInRow - 1) * formationWidthGap;

      for (let index = 0; index < shipsInRow; index += 1) {
        const localY = index * formationWidthGap - rowWidth * 0.5;
        this.drawShipGlyph(localX, localY);
      }

      remaining -= shipsInRow;
      rowSize += 1;
      rowIndex += 1;
    }

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
