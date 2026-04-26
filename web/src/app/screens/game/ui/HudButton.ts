import { Container, Graphics, Text } from "pixi.js";

import { palette } from "../theme";

interface HudButtonOptions {
  label: string;
  width: number;
  height: number;
  tint?: number;
  onPress: () => void;
}

export class HudButton extends Container {
  private readonly background = new Graphics();
  private readonly labelText: Text;
  private readonly widthValue: number;
  private readonly heightValue: number;
  private readonly tintValue: number;
  private disabled = false;

  constructor(options: HudButtonOptions) {
    super();

    this.widthValue = options.width;
    this.heightValue = options.height;
    this.tintValue = options.tint ?? palette.surfaceAlt;
    this.labelText = new Text({
      text: options.label,
      anchor: 0.5,
      style: {
        fill: palette.text,
        fontFamily: "Trebuchet MS",
        fontSize: 16,
        fontWeight: "700",
      },
    });

    this.eventMode = "static";
    this.cursor = "pointer";
    this.addChild(this.background, this.labelText);
    this.on("pointertap", () => {
      if (!this.disabled) {
        options.onPress();
      }
    });
    this.on("pointerover", () => {
      if (!this.disabled) {
        this.alpha = 1;
      }
    });
    this.on("pointerout", () => {
      if (!this.disabled) {
        this.alpha = 0.92;
      }
    });

    this.alpha = 0.92;
    this.draw();
  }

  public setDisabled(value: boolean): void {
    this.disabled = value;
    this.eventMode = value ? "none" : "static";
    this.cursor = value ? "default" : "pointer";
    this.alpha = value ? 0.45 : 0.92;
  }

  private draw(): void {
    this.background.clear();
    this.background.roundRect(
      -this.widthValue * 0.5,
      -this.heightValue * 0.5,
      this.widthValue,
      this.heightValue,
      14,
    );
    this.background.fill({ color: this.tintValue, alpha: 0.92 });
    this.background.roundRect(
      -this.widthValue * 0.5,
      -this.heightValue * 0.5,
      this.widthValue,
      this.heightValue,
      14,
    );
    this.background.stroke({ color: palette.outline, width: 2, alpha: 1 });
  }
}
