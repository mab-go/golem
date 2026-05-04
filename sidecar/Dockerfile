# Builder: compile TypeScript to JavaScript.
FROM node:22-bookworm-slim AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY tsconfig.json tsconfig.base.json tsconfig.app.json tsconfig.gen.json ./
COPY src/ src/

RUN npm run build

# Runtime: Playwright image ships Chromium + system deps. Needed for the
# prismarine-viewer → headless Chromium screenshot pipeline.
FROM mcr.microsoft.com/playwright:v1.49.0-jammy

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --omit=dev

# Playwright's base image has Chromium, but installing via the Playwright CLI
# ensures the browser download matches the playwright npm package version.
RUN npx playwright install chromium

COPY --from=builder /app/dist/ dist/

# Playwright's image already ships a non-root `pwuser`. Run as that user.
USER pwuser
EXPOSE 50051
ENTRYPOINT ["node", "dist/index.js"]
