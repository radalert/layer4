FROM golang:latest

# Copy the app
ADD . /app
WORKDIR /app
# Build it
ENV GOPATH /app/_vendor
RUN go test -v
RUN go build -v taut.go
# Run it
CMD ./taut
