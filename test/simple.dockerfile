FROM alpine:latest
RUN echo "Building with shmocker!"
RUN apk add --no-cache curl
CMD ["sh", "-c", "echo 'Hello from shmocker!'"]