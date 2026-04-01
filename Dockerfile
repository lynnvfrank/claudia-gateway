# Claudia Gateway (v0.1)
#
# Purpose: HTTP “front door” for IDE clients (e.g. VS Code Continue). Validates
# gateway-issued API tokens, exposes OpenAI-compatible /v1/models and
# /v1/chat/completions, prepends the virtual model Claudia-<semver>, applies
# routing policy + sequential LiteLLM fallback for that id, and proxies explicit
# LiteLLM model ids straight through. Upstream calls use the LiteLLM proxy
# master key from the environment (see docker-compose.yml).
#
# Base: Node 22 Alpine. Multi-stage: compile TypeScript, prune devDependencies.
# Exposed port: 3000 (override via gateway.yaml listen_port + compose mapping).
# Default CMD: node dist/index.js — operators override via env
# (LOG_LEVEL, LITELLM_MASTER_KEY, CLAUDIA_GATEWAY_CONFIG) and mounted config/.

FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY tsconfig.json ./
COPY src ./src
RUN npm run build && npm prune --omit=dev

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
RUN apk add --no-cache wget
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
EXPOSE 3000
CMD ["node", "dist/index.js"]
