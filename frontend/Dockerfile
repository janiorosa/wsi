# syntax=docker/dockerfile:1
FROM node:22.11.0-bookworm-slim AS construcao
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY . .
RUN npm run build

FROM nginx:1.27.2-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=construcao /app/dist /usr/share/nginx/html
EXPOSE 80
