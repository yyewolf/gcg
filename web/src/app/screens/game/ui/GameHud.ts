import { Container, Graphics, Text } from "pixi.js";

import { palette } from "../theme";

export interface HudViewModel {
  sendPercentage: number;
}

export class GameHud extends Container {
  private readonly chrome = new Graphics();
  private readonly caption = new Text({
    text: "SEND",
    anchor: { x: 1, y: 0 },
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 11,
      fontWeight: "700",
      letterSpacing: 1.5,
    },
  });
  private readonly percentage = new Text({
    anchor: { x: 1, y: 0 },
    style: {
      fill: palette.accent,
      fontFamily: "Trebuchet MS",
      fontSize: 22,
      fontWeight: "700",
    },
  });
  private readonly hint = new Text({
    text: "Shift + Wheel",
    anchor: { x: 1, y: 0 },
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 11,
      fontWeight: "600",
    },
  });

  constructor() {
    super();

    this.addChild(this.chrome, this.caption, this.percentage, this.hint);
  }

  public resize(width: number, height: number): void {
    this.chrome.clear();
    const panelWidth = 132;
    const panelHeight = 58;
    const panelX = width - panelWidth - 20;
    const panelY = Math.max(12, Math.min(16, height - panelHeight - 12));

    this.chrome.roundRect(panelX, panelY, panelWidth, panelHeight, 18);
    this.chrome.fill({ color: palette.panel, alpha: 0.9 });
    this.chrome.roundRect(panelX, panelY, panelWidth, panelHeight, 18);
    this.chrome.stroke({ color: palette.outline, width: 1.5, alpha: 1 });

    this.caption.position.set(panelX + panelWidth - 14, panelY + 10);
    this.percentage.position.set(panelX + panelWidth - 14, panelY + 22);
    this.hint.position.set(panelX + panelWidth - 14, panelY + 42);
  }

  public render(model: HudViewModel): void {
    this.percentage.text = `${model.sendPercentage}%`;
  }
}
