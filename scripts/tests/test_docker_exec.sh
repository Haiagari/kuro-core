docker run -d --name proxy-test ghcr.io/Haiagari/kuro-git-proxy:latest sleep 3600
docker exec proxy-test id
docker rm -f proxy-test
