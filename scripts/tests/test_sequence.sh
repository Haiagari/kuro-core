# wipe
docker run --rm -v /tmp:/host_tmp alpine sh -c "rm -rf /host_tmp/kuro-test-seq && mkdir -p /host_tmp/kuro-test-seq && chmod 777 /host_tmp/kuro-test-seq"
# start container
docker run -d --name proxy-seq-test -v /tmp/kuro-test-seq:/tmp/kuro-scans ghcr.io/Haiagari/kuro-git-proxy:latest sleep 3600
# step 1: test_response_format
docker exec proxy-seq-test mkdir -p /tmp/kuro-scans/e2e-test/format-11655
# simulate worker creating file inside as root!
docker run --rm -v /tmp/kuro-test-seq:/tmp/kuro-scans alpine touch /tmp/kuro-scans/e2e-test/format-11655/file.txt
# cleanup step (simulate git-proxy deleting the folder)
# wait, git-proxy runs rm -rf
docker exec proxy-seq-test rm -rf /tmp/kuro-scans/e2e-test/format-11655 || echo "rm failed"
# step 2: test_proxy_scan_clean
docker exec proxy-seq-test mkdir -p /tmp/kuro-scans/e2e-test/clean-11655 || echo "mkdir clean failed"
# ls
docker exec proxy-seq-test ls -ld /tmp/kuro-scans/e2e-test
# rm container
docker rm -f proxy-seq-test
