FROM golang:1.25.6-alpine
RUN go install github.com/mitranim/gow@latest
RUN apk add make

# Set the working directory inside the container
WORKDIR /app
# Copy the go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download
