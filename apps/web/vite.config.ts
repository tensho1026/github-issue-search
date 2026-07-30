import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (
            id.includes("/node_modules/react-hook-form/") ||
            id.endsWith("/features/profile/components/ProfileSearchForm.tsx")
          ) {
            return "form-runtime";
          }
          if (
            id.includes("/src/components/ui/") ||
            id.includes("/src/shared/") ||
            id.includes("/src/features/issue-search/model/") ||
            id.includes("/node_modules/@tanstack/")
          ) {
            return "app-shared";
          }
        },
      },
    },
  },
  plugins: [react(), tailwindcss()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
  },
  test: {
    clearMocks: true,
    coverage: {
      exclude: ["src/main.tsx", "src/test/**"],
      include: ["src/**/*.{ts,tsx}"],
      provider: "v8",
      reporter: ["text", "json-summary", "lcov"],
      reportsDirectory: "./coverage",
    },
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["./src/test/setup.ts"],
  },
});
