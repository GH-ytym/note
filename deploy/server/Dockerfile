FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -g 10001 note && adduser -D -u 10001 -G note note
WORKDIR /app
COPY --chmod=755 note-api /app/note-api
COPY web /app/web
RUN mkdir /data && chown note:note /data
USER 10001:10001
ENV HTTP_ADDR=0.0.0.0:8080 NOTE_DB_PATH=/data/note.db NOTE_WEB_DIR=/app/web NOTE_COOKIE_SECURE=true
EXPOSE 8080
ENTRYPOINT ["/app/note-api"]
