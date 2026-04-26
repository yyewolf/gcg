import { setEngine } from "./app/getEngine";
import { LoadScreen } from "./app/screens/LoadScreen";
import { GameScreen } from "./app/screens/game/GameScreen";
import { userSettings } from "./app/utils/userSettings";
import { CreationEngine } from "./engine/engine";

/**
 * Importing these modules will automatically register there plugins with the engine.
 */
import "@pixi/sound";
// import "@esotericsoftware/spine-pixi-v8";

// Create a new creation engine instance
const engine = new CreationEngine();
setEngine(engine);

(async () => {
  // Initialize the creation engine instance
  await engine.init({
    background: "#030712",
    resizeOptions: { minWidth: 960, minHeight: 640, letterbox: false },
  });

  // Initialize the user settings
  userSettings.init();

  // Show the load screen
  await engine.navigation.showScreen(LoadScreen);
  // Show the main screen once the load screen is dismissed
  await engine.navigation.showScreen(GameScreen);
})();
