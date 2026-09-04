docker run --rm -v /tmp:/host_tmp alpine sh -c "rm -rf /host_tmp/kuro-test-bind && mkdir -p /host_tmp/kuro-test-bind && chmod 777 /host_tmp/kuro-test-bind"
docker run -d --name proxy-mkdir-test -v /tmp/kuro-test-bind:/tmp/kuro-scans ghcr.io/Haiagari/kuro-git-proxy:latest sleep 3600
docker exec proxy-mkdir-test sh -c "mkdir -p /tmp/kuro-scans/e2e-test/a && ls -ld /tmp/kuro-scans/e2e-test"
docker exec proxy-mkdir-test sh -c "mkdir -p /tmp/kuro-scans/e2e-test/b"
docker rm -f proxy-mkdir-test
