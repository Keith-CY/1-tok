ARG BUN_VERSION=1.2.22
# Prefer passing --build-arg BUN_VERSION=$(grep -o 'bun@[^"]*' package.json | cut -d'@' -f2)
# to keep package.json as the single source of truth.

FROM oven/bun:${BUN_VERSION} AS deps

WORKDIR /app
COPY package.json bun.lockb tsconfig.base.json ./
COPY apps/web/package.json apps/web/package.json
COPY packages/contracts/package.json packages/contracts/package.json
RUN bun install --frozen-lockfile

FROM oven/bun:${BUN_VERSION} AS builder

WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN bun --cwd apps/web build

FROM oven/bun:${BUN_VERSION} AS runner

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000

COPY --from=builder /app/apps/web/.next/standalone ./
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public

EXPOSE 3000

CMD ["bun", "apps/web/server.js"]
