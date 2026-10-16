FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/waterer ./cmd/waterer \
 && GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/simulator ./cmd/simulator
# empty dir owned by the distroless nonroot uid so the named volume inherits it
RUN mkdir /out/data && chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot AS waterer
COPY --from=build /out/waterer /waterer
COPY --from=build --chown=65532:65532 /out/data /data
ENV WATERER_DB=/data/waterer.db
EXPOSE 8080
ENTRYPOINT ["/waterer"]

FROM gcr.io/distroless/static-debian12:nonroot AS simulator
COPY --from=build /out/simulator /simulator
COPY deploy/simulator.yaml /etc/simulator.yaml
ENTRYPOINT ["/simulator"]
