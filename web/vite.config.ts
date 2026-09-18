import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// Baseline icon references for the shell (kept minimal).
const apiOrigin = process.env.SDT_VIEWER_ORIGIN ?? "http://localhost:8443";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Dev-time only: the production build is served by sdtviewer (go:embed),
      // where /api/* originates from the same origin.
      "/api": {
        target: apiOrigin,
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: "node",
    setupFiles: ["src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
