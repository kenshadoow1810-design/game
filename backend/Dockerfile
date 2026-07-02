FROM golang:1.21-alpine

WORKDIR /app

# Instalar dependências
RUN apk add --no-cache git

# Copiar go.mod e go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copiar código
COPY . .

# Expor porta
EXPOSE 8080

# Comando para rodar
CMD ["go", "run", "main.go"]