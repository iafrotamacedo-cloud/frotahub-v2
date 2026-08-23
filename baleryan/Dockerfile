# rev 1 — como o Render compila e roda o baleryan
#
# Duas etapas: a primeira compila, a segunda só carrega o resultado. A imagem final
# tem o binário e mais nada — nem o Go, nem o código-fonte. Sobe em milissegundos,
# que é a razão de o motor ser em Go (o plano gratuito do Render hiberna o serviço).

# ---------- etapa 1: compilar ----------
FROM golang:1.24-alpine AS construir
WORKDIR /obra

# As dependências primeiro, sozinhas: assim o Docker reaproveita esta camada
# enquanto o go.mod não mudar, e recompilar fica rápido.
COPY go.mod ./
RUN go mod download

COPY . .

# CGO desligado e binário estático: roda na imagem vazia da etapa 2.
# -trimpath tira os caminhos da máquina de compilação do binário.
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /baleryan ./cmd/baleryan

# ---------- etapa 2: rodar ----------
FROM alpine:3.20
# Certificados: sem eles o motor não consegue falar HTTPS com o Supabase.
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 motor
USER motor
COPY --from=construir /baleryan /baleryan
EXPOSE 8000
ENTRYPOINT ["/baleryan"]
