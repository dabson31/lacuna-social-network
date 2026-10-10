# the frontend image, the pair to src/server/Dockerfile.
FROM node:22-alpine AS deps

WORKDIR /build/src/web

# deps as their own layer so docker caches them
COPY src/web/package.json src/web/package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build

WORKDIR /build/src/web

COPY --from=deps /build/src/web/node_modules ./node_modules
COPY src/web ./
COPY src/shared /build/src/shared

ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL

RUN npm run build

FROM node:22-alpine AS runtime

LABEL org.opencontainers.image.title="social-network-frontend"
LABEL org.opencontainers.image.description="Next.js client for the social network, served on :3000"

# never run as root, and a fixed uid keeps file ownership stable, same as the backend image
RUN addgroup -g 10002 -S app && adduser -u 10002 -S app -G app

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# the three pieces a standalone server needs: its own tree, the static chunks, and public/.
# server.js expects .next/static and public to sit beside it, which is why they are copied
COPY --from=build --chown=app:app /build/src/web/.next/standalone/src/web/ ./
COPY --from=build --chown=app:app /build/src/web/.next/static ./.next/static
COPY --from=build --chown=app:app /build/src/web/public ./public

USER app
EXPOSE 3000

#? /login is the one route that renders without a session, so it is the honest thing to probe:
# a 200 here means next is serving and the bundle resolved, not merely that node is alive
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=5 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:3000/login || exit 1

CMD ["node", "server.js"]
