# Docker Watch

Uses [Prometheus Node Exporter](https://github.com/prometheus/node_exporter) as a base. Removed all node_exporter collectors and added two for docker.

These collectors hit the Stats and Inspect API's. I've only picked out the few metrics which are interesting to me, adding more should be trivial.

This is all pretty hacky, so pull requests welcome.

## Docker

```bash
docker run \
  --rm \
  -p 9100:9100 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  alexrudd/docker_watch
```

```bash
curl localhost:9100/metrics
```

## Building

Included a small build script which statically compiles the Go binary and runs the Docker build command

```bash
./build.sh
```
