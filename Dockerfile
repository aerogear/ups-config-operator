FROM centos:7
WORKDIR /app
ADD ./main /app/main
CMD ["./main"]
