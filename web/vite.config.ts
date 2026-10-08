import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// Baseline icon references for the shell (kept minimal).
const apiOrigin = process.env.SDT_VIEWER_ORIGIN ?? "http://localhost:8443";

// marp-core statically requires mathjax-full even with `math: 'katex'`; the
// viewer never selects the MathJax renderer, so stub it to keep several MB out
// of the lazy slide chunk (see src/stubs/mathjax-full.ts).
const mathjaxStub = new URL("./src/stubs/mathjax-full.ts", import.meta.url).pathname;

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [{ find: /^mathjax-full(\/.*)?$/, replacement: mathjaxStub }],
  },
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
  build: {
    // The ported three.js engine, marp, elk and cytoscape are intentionally
    // large lazy chunks; the graph/map routes load on demand (analysis
    // 20261007-074001, W1), so the default 500 kB warning is noise here.
    chunkSizeWarningLimit: 2000,
  },
  test: {
    environment: "node",
    setupFiles: ["src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text-summary", "lcov"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/test/**", "src/stubs/**", "src/env.d.ts"],
    },
  },
});
