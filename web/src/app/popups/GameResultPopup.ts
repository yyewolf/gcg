import { animate } from "motion";
import { BlurFilter, Container, Graphics, Text } from "pixi.js";

import { palette } from "../screens/game/theme";
import { HudButton } from "../screens/game/ui/HudButton";

export type GameResult = "win" | "lose";

export class GameResultPopup extends Container {
  private readonly bg: Graphics;
  private readonly panel: Container;
  private readonly panelBase: Graphics;
  private readonly title: Text;
  private readonly subtitle: Text;
  private readonly doneButton: HudButton;
  private widthValue = 0;
  private heightValue = 0;
  private result: GameResult = "lose";

  constructor(onDismiss: () => void) {
    super();

    this.visible = false;

    this.bg = new Graphics();
    this.bg.eventMode = "static";
    this.addChild(this.bg);

    this.panel = new Container();
    this.addChild(this.panel);

    this.panelBase = new Graphics();
    this.panel.addChild(this.panelBase);

    this.title = new Text({
      text: "Defeat",
      anchor: 0.5,
      style: {
        fill: 0xec5b5b,
        fontFamily: "Trebuchet MS",
        fontSize: 50,
        fontWeight: "700",
      },
    });
    this.title.y = -78;
    this.panel.addChild(this.title);

    this.subtitle = new Text({
      text: "You lost the match.",
      anchor: 0.5,
      style: {
        fill: palette.text,
        fontFamily: "Trebuchet MS",
        fontSize: 24,
        fontWeight: "600",
        align: "center",
        wordWrap: true,
      },
    });
    this.subtitle.y = 4;
    this.panel.addChild(this.subtitle);

    this.doneButton = new HudButton({
      label: "Back to Lobby",
      width: 220,
      height: 54,
      tint: palette.surfaceAlt,
      onPress: onDismiss,
    });
    this.doneButton.y = 92;
    this.panel.addChild(this.doneButton);

    this.applyResult();
  }

  public setResult(result: GameResult): void {
    this.result = result;
    this.applyResult();
  }

  public async present(): Promise<void> {
    this.visible = true;
    await this.show();
  }

  public async dismiss(): Promise<void> {
    await this.hide();
    this.visible = false;
  }

  public resize(width: number, height: number) {
    this.widthValue = width;
    this.heightValue = height;

    this.bg.clear();
    this.bg.rect(0, 0, width, height);
    this.bg.fill({ color: 0x0, alpha: 0.8 });

    this.panelBase.clear();
    this.panelBase.roundRect(-210, -160, 420, 320, 28);
    this.panelBase.fill({ color: palette.panel, alpha: 0.98 });
    this.panelBase.roundRect(-210, -160, 420, 320, 28);
    this.panelBase.stroke({ color: palette.outline, width: 2, alpha: 1 });

    this.panel.x = width * 0.5;
    this.panel.y = height * 0.5;
    this.subtitle.style.wordWrapWidth = 320;
  }

  public async show() {
    this.filters = [new BlurFilter({ strength: 0 })];
    this.resize(this.widthValue, this.heightValue);
    this.bg.alpha = 0;
    this.panel.pivot.y = -400;
    animate(this.bg, { alpha: 0.8 }, { duration: 0.2, ease: "linear" });
    await animate(
      this.panel.pivot,
      { y: 0 },
      { duration: 0.3, ease: "backOut" },
    );
  }

  public async hide() {
    animate(this.bg, { alpha: 0 }, { duration: 0.2, ease: "linear" });
    await animate(
      this.panel.pivot,
      { y: -500 },
      { duration: 0.3, ease: "backIn" },
    );
    this.filters = [];
  }

  private applyResult(): void {
    const won = this.result === "win";
    this.title.text = won ? "Victory" : "Defeat";
    this.title.style.fill = won ? 0x36d98a : 0xec5b5b;
    this.subtitle.text = won ? "You won the match." : "You lost the match.";
  }
}
