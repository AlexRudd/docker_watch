CGO_ENABLED=0 GOOS=linux go build -o docker_watch
docker build -t alexrudd/docker_watch .
