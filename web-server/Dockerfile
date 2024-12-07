FROM alpine:3.21.0
ARG TARGETOS
ARG TARGETARCH
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY /${TARGETOS}/${TARGETARCH} /
COPY /dist /dist
RUN chown -R appuser:appgroup /dist
USER appuser
EXPOSE 8080/tcp
ENTRYPOINT ["/sdtserve"]
