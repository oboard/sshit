import { fileURLToPath, URL } from "node:url";

import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify("sshit"),
  },
  plugins: [svelte()],
  resolve: {
    alias: {
      $app: fileURLToPath(new URL("./src/app", import.meta.url)),
      $lib: fileURLToPath(new URL("./src/lib", import.meta.url)),
    },
  },
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
});
