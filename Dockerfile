FROM --platform=$BUILDPLATFORM madebytimo/nodejs AS frontend
WORKDIR /media/workdir
COPY frontend/ ./
RUN npm install \
    && npm run build

FROM --platform=$BUILDPLATFORM madebytimo/go AS backend
WORKDIR /media/workdir
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/subprime ./cmd/subprime \
    && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/healthcheck ./cmd/healthcheck

FROM madebytimo/base

WORKDIR /app
COPY --from=backend /out/subprime /out/healthcheck /usr/local/bin/
COPY --from=frontend /media/workdir/build /app/static

ENV LOG_LEVEL="info"
ENV PORT="8080"
ENV STATIC_DIR="/app/static"

USER user
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD [ "healthcheck" ]

CMD [ "subprime" ]

LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/DT-Entertainment-Studios/subprime-game"
