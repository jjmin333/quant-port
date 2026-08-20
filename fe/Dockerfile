# syntax=docker/dockerfile:1

FROM node:20-bookworm-slim AS deps
WORKDIR /app
COPY package.json package-lock.json ./
# npm ci는 lockfile 그대로 설치한다 — 빌드마다 의존성이 미끄러지지 않는다.
RUN npm ci

FROM node:20-bookworm-slim AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# NEXT_PUBLIC_* 는 빌드 시점에 번들에 박힌다. 브라우저는 nginx와 같은 오리진으로
# 요청하므로(getApiBaseUrl()이 브라우저에서 "" 를 반환) 여기서 줄 값이 없다.
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:20-bookworm-slim AS runtime
WORKDIR /app

ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

RUN useradd -r -u 10001 -g root nextjs

# standalone 출력은 서버와 추려낸 node_modules만 담는다. static과 public은
# 포함되지 않으므로 따로 복사해야 한다 — 빠뜨리면 CSS·JS·이미지가 404가 된다.
COPY --from=build --chown=nextjs:root /app/.next/standalone ./
COPY --from=build --chown=nextjs:root /app/.next/static ./.next/static
COPY --from=build --chown=nextjs:root /app/public ./public

USER nextjs

EXPOSE 3000

# standalone 서버는 next CLI가 아니라 생성된 server.js로 뜬다.
CMD ["node", "server.js"]
