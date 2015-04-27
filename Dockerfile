FROM golang:latest

# Copy the app
ADD . /app
WORKDIR /app
# Build it
RUN go build -v nudger.go
# Run it
CMD ./nudger
