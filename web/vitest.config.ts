import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    // Components read window.location for routing and call window.location.href
    // on navigation, so each file gets a fresh jsdom rather than sharing one.
    restoreMocks: true,
  },
});
