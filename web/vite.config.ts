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
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    hmr: {
      // The browser connects through sshit's public port; Vite's HTTP and HMR
      // traffic therefore share that origin via the backend reverse proxy.
      clientPort: Number(process.env.SSHIT_PORT ?? 2222),
      path: "/__vite_hmr",
    },
  },
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
    // MarkdownEditor is deliberately lazy-loaded and its 505 kB minified chunk
    // is expected because it bundles CodeMirror's editing extensions.
    chunkSizeWarningLimit: 550,
  },
});
