import { Container, Graphics, Text } from "pixi.js";

import { palette } from "../theme";

export interface HudViewModel {
  sendPercentage: number;
  playerColor: number;
  playerId: number;
}

export class GameHud extends Container {
  private readonly identityChrome = new Graphics();
  private readonly identitySwatch = new Graphics();
  private identitySwatchX = 34;
  private identitySwatchY = 29;
  private readonly identityCaption = new Text({
    text: "YOUR COLOR",
    anchor: { x: 0, y: 0 },
    style: {
      fill: palette.mutedText,
      fontFamily: "Trebuchet MS",
      fontSize: 11,
      fontWeight: "700",
      letterSpacing: 1.4,
    },
  });
  private readonly identityLabel = new Text({
    anchor: { x: 0, y: 0 },
    style: {
      fill: palette.text,
      fontFamily: "Trebuchet MS",
      fontSize: 18,
      fontWeight: "700",
    },
  });
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

    this.addChild(
      this.identityChrome,
      this.identitySwatch,
      this.identityCaption,
      this.identityLabel,
      this.chrome,
      this.caption,
      this.percentage,
      this.hint,
    );
  }

  public resize(width: number, height: number): void {
    const panelY = Math.max(12, height - 58 - 12);

    this.identityChrome.clear();
    const identityWidth = 168;
    const identityHeight = 58;
    const identityX = 20;
    this.identityChrome.roundRect(
      identityX,
      panelY,
      identityWidth,
      identityHeight,
      18,
    );
    this.identityChrome.fill({ color: palette.panel, alpha: 0.9 });
    this.identityChrome.roundRect(
      identityX,
      panelY,
      identityWidth,
      identityHeight,
      18,
    );
    this.identityChrome.stroke({
      color: palette.outline,
      width: 1.5,
      alpha: 1,
    });

    this.identityCaption.position.set(identityX + 48, panelY + 10);
    this.identityLabel.position.set(identityX + 48, panelY + 25);
    this.identitySwatchX = identityX + 18;
    this.identitySwatchY = panelY + identityHeight * 0.5;

    this.chrome.clear();
    const panelWidth = 132;
    const panelHeight = 58;
    const panelX = width - panelWidth - 20;

    this.chrome.roundRect(panelX, panelY, panelWidth, panelHeight, 18);
    this.chrome.fill({ color: palette.panel, alpha: 0.9 });
    this.chrome.roundRect(panelX, panelY, panelWidth, panelHeight, 18);
    this.chrome.stroke({ color: palette.outline, width: 1.5, alpha: 1 });

    this.caption.position.set(panelX + panelWidth - 14, panelY + 10);
    this.percentage.position.set(panelX + panelWidth - 14, panelY + 22);
    this.hint.position.set(panelX + panelWidth - 14, panelY + 42);
  }

  public render(model: HudViewModel): void {
    this.identityLabel.text = `P${model.playerId}`;
    this.identitySwatch.clear();
    this.identitySwatch.circle(this.identitySwatchX, this.identitySwatchY, 12);
    this.identitySwatch.fill({ color: model.playerColor, alpha: 0.94 });
    this.identitySwatch.circle(this.identitySwatchX, this.identitySwatchY, 12);
    this.identitySwatch.stroke({ color: 0xffffff, width: 2, alpha: 0.5 });

    this.percentage.text = `${model.sendPercentage}%`;
  }
}
