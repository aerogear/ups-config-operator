FROM fedora:latest
WORKDIR /app
ADD ./main /app/main
CMD ["./main"]
