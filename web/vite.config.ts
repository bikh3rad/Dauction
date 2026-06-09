import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

// Dauction frontend — Vite + React + TS.
// Dev server proxies /apis to the gateway so the browser hits a same-origin path
// (avoids CORS) while the gateway stays the single edge. Override the target with
// VITE_GATEWAY_PROXY in .env.local if the gateway runs elsewhere.
export default defineConfig(({ mode }) => {
  const gateway = process.env.VITE_GATEWAY_PROXY || "http://localhost:18080";
  return {
    plugins: [react()],
    resolve: {
      alias: { "@": path.resolve(__dirname, "src") },
    },
    server: {
      port: 5173,
      host: true,
      proxy: {
        "/apis": {
          target: gateway,
          changeOrigin: true,
          ws: true, // pass through notifier WS/SSE upgrades
        },
      },
    },
    preview: { port: 4173, host: true },
  };
});
