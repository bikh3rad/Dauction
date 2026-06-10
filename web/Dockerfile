# Dauction web — build the static bundle and serve it with `vite preview`,
# which also proxies /apis to the gateway (see preview.proxy in vite.config.ts).
#
# Single stage on node:20-alpine so the offline host can build WITHOUT pulling a
# second base image (alpine node is already cached; nginx/bookworm are not and
# the registry mirror is flaky). The build still needs the npm registry — on an
# offline host build via the proxy override (deploy/docker-compose.proxy.yml),
# which injects HTTP(S)_PROXY and runs with `network: host`, exactly like the
# Go services. npm reads the predefined HTTP_PROXY/HTTPS_PROXY ARGs.

ARG NODE_IMAGE=node:20-alpine
FROM ${NODE_IMAGE}
WORKDIR /src

# Deps first for layer caching. `npm ci` when the lock is in sync, else a plain
# install so an out-of-sync lock never blocks the image.
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund

COPY . .
# VITE_USE_MOCK is baked at build time (Vite inlines import.meta.env). When
# "true", the SPA serves the full in-memory sample catalog (8 lots + imagery,
# passive auctions, vaults, admin) regardless of the backend — the demo build.
# Set to "false" to talk to the live gateway. Default is provided by compose.
ARG VITE_USE_MOCK
ENV VITE_USE_MOCK=${VITE_USE_MOCK}
# Full build = `tsc -b && vite build` (verified green). Produces /src/dist.
RUN npm run build

EXPOSE 8080
# preview serves dist/ on :8080 and proxies /apis -> $VITE_GATEWAY_PROXY.
CMD ["node", "node_modules/vite/bin/vite.js", "preview", "--host", "--port", "8080"]
