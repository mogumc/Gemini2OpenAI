FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY gemini2openai .

EXPOSE 8080

ENTRYPOINT ["./gemini2openai"]
