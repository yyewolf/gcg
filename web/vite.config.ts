import { defineConfig } from "vite";

import { assetpackPlugin } from "./scripts/assetpack-vite-plugin";

// https://vite.dev/config/
export default defineConfig({
  plugins: [assetpackPlugin()],
  server: {
    port: 5173,
    open: true,
    proxy: {
      "/ws": {
        target: "ws://127.0.0.1:8080",
        ws: true,
      },
    },
  },
  define: {
    APP_VERSION: JSON.stringify(process.env.npm_package_version),
  },
});
