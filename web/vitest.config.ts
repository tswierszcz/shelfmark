import { defineConfig } from "vitest/config";

// Kept apart from vite.config.ts on purpose: the reactRouter() plugin expects
// the React Router build/dev pipeline and does not run under Vitest. Routes
// fetch in useEffect (no loaders), so tests render them directly.
export default defineConfig({
  resolve: {
    tsconfigPaths: true,
  },
  test: {
    environment: "jsdom",
    include: ["app/**/*.test.{ts,tsx}"],
    setupFiles: ["app/test/setup.ts"],
    restoreMocks: true,
  },
});
