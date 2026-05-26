FROM gcr.io/distroless/static-debian12

ARG TARGETOS
ARG TARGETARCH

COPY gemini2openai-${TARGETOS}-${TARGETARCH} /gemini2openai

ENV TZ=Asia/Shanghai

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/gemini2openai"]