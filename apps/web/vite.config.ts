import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";
import { hostedPublicConfig } from "./public-config";

export default defineConfig(({ mode }) => ({
  plugins: [react()],
  base: "/",
  envDir: mode === "hosted" ? false : undefined,
  ...(mode === "hosted"
    ? {
        define: Object.fromEntries(
          Object.entries(hostedPublicConfig(process.env)).map(
            ([key, value]) => [`import.meta.env.${key}`, JSON.stringify(value)],
          ),
        ),
      }
    : {}),
  build: { sourcemap: false },
  test: {
    include: ["src/**/*.test.{ts,tsx}"],
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
  },
}));
