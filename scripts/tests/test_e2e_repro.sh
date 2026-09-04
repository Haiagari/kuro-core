docker run --rm -v /tmp:/host_tmp alpine sh -c "rm -rf /host_tmp/kuro-test-repro && mkdir -p /host_tmp/kuro-test-repro && chmod 777 /host_tmp/kuro-test-repro"

# start container with same image
docker run -d --name proxy-repro-test -v /tmp/kuro-test-repro:/tmp/kuro-scans ghcr.io/Haiagari/kuro-git-proxy:latest sleep 3600

# Write script into container
docker exec -u root proxy-repro-test sh -c "echo 'mkdir -p /tmp/kuro-scans/e2e-test/format-11655; rm -rf /tmp/kuro-scans/e2e-test/format-11655; mkdir -p /tmp/kuro-scans/e2e-test/clean-11655' > /tmp/test.sh"

# run script
docker exec proxy-repro-test bash -e /tmp/test.sh || echo "failed"

# ls
docker exec proxy-repro-test ls -ld /tmp/kuro-scans/e2e-test

docker rm -f proxy-repro-test
