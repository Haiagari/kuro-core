docker run -d --name proxy-mkdir-test ghcr.io/Haiagari/kuro-git-proxy:latest sleep 3600
docker exec proxy-mkdir-test sh -c "mkdir -p /tmp/e2e-test/a && ls -ld /tmp/e2e-test"
docker exec proxy-mkdir-test sh -c "mkdir -p /tmp/e2e-test/b"
docker rm -f proxy-mkdir-test
