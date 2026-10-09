# the frontend image, the pair to src/server/Dockerfile.
#! the build context is the REPO ROOT and not src/web, because tsconfig's @shared/* alias
#! points at src/shared, a sibling of src/web. building from src/web means the alias cannot
#! resolve and the typescript build fails on an import that works fine in dev
FROM node:22-alpine AS deps

WORKDIR /build/src/web

# deps as their own layer so docker caches them, the same shape as the go image's go mod download
COPY src/web/package.json src/web/package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build

#! the build stage mirrors the repo layout exactly — src/web beside src/shared — rather than
#! flattening the app to /app. two reasons, and the second one bites silently:
#!   1. ../shared has to resolve the same way it does in development
#!   2. next's standalone output is emitted at a path RELATIVE TO outputFileTracingRoot, which
#!      next.config.ts sets to the repo root. so the bundle lands at
#!      .next/standalone/src/web/, not .next/standalone/. flatten the build and that path
#!      changes, and the runtime stage copies an empty directory and dies on "cannot find
#!      module /app/server.js"
WORKDIR /build/src/web

COPY --from=deps /build/src/web/node_modules ./node_modules
COPY src/web ./
COPY src/shared /build/src/shared

# NEXT_PUBLIC_* is inlined into the browser bundle at BUILD time, not read at runtime.
#! so it has to be right here. it is the url the READER'S BROWSER uses, which is why it is
#! localhost:8080 and never backend:8080 — none of this app's fetching runs in the container
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL

RUN npm run build

FROM node:22-alpine AS runtime

# never run as root, and a fixed uid keeps file ownership stable, same as the backend image
RUN addgroup -g 10002 -S app && adduser -u 10002 -S app -G app

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# the three pieces a standalone server needs: its own tree, the static chunks, and public/.
# server.js expects .next/static and public to sit beside it, which is why they are copied
# separately — the standalone trace deliberately leaves them out
COPY --from=build --chown=app:app /build/src/web/.next/standalone/src/web/ ./
COPY --from=build --chown=app:app /build/src/web/.next/static ./.next/static
COPY --from=build --chown=app:app /build/src/web/public ./public

USER app
EXPOSE 3000

CMD ["node", "server.js"]
