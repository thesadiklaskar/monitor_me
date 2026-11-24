FROM node:26.13.0-alpine AS dependencies

WORKDIR /app

COPY package.json ./
RUN npm install

FROM node:26.13.0-alpine AS build

WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=dependencies /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:26.13.0-alpine AS runtime

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN addgroup -S -g 10001 web && \
    adduser -S -D -H -u 10001 -G web web

COPY --from=build --chown=web:web /app/.next/standalone ./
COPY --from=build --chown=web:web /app/.next/static ./.next/static
COPY --from=build --chown=web:web /app/public ./public

USER 10001:10001

EXPOSE 3000

CMD ["node", "server.js"]
