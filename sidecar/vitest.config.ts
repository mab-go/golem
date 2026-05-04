import { defineConfig } from "vitest/config"

export default defineConfig({
  test: {
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      include: ["src/**/*.ts"],
      exclude: ["src/grpc/generated/**", "src/index.ts"],
      reporter: ["text", "html"],
      reportsDirectory: "coverage",
    },
  },
})
