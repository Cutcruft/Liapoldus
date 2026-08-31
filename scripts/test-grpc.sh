#!/usr/bin/env bash
set -euo pipefail

grpcurl_bin="${GRPCURL_BIN:-grpcurl}"
address="${LIAPOLDUS_GRPC_ADDR:-127.0.0.1:8080}"
proto_root="${LIAPOLDUS_PROTO_ROOT:-proto}"
proto_file="liapoldus/management/v1/management.proto"

grpc() {
  local method="$1"
  shift
  "${grpcurl_bin}" -plaintext \
    -import-path "${proto_root}" \
    -proto "${proto_file}" \
    "$@" "${address}" "${method}"
}

echo "Checking gRPC management API at ${address}"

suffix="$(date +%s%N)"
site_json="$(printf '{"name":"WSL PostgreSQL site","slug":"wsl-postgres-site-%s"}' "${suffix}" \
  | grpc liapoldus.management.v1.ManagementService/CreateSite -d @)"
site_id="$(jq -r '.site.id' <<<"${site_json}")"
test -n "${site_id}"

page_json="$(printf '{"siteId":"%s","name":"Home","slug":"home","root":{"id":"root","type":"Container","children":[{"id":"title","type":"Text","props":{"text":"Hello WSL"}}]}}' "${site_id}" \
  | grpc liapoldus.management.v1.ManagementService/CreatePage -d @)"
page_id="$(jq -r '.page.id' <<<"${page_json}")"
test -n "${page_id}"
test "$(jq -r '.page.version' <<<"${page_json}")" -eq 1

updated_json="$(printf '{"id":"%s","root":{"id":"root","type":"Container","children":[]}}' "${page_id}" \
  | grpc liapoldus.management.v1.ManagementService/UpdatePageTree -d @)"
test "$(jq -r '.page.version' <<<"${updated_json}")" -eq 2

versions_json="$(printf '{"pageId":"%s"}' "${page_id}" \
  | grpc liapoldus.management.v1.ManagementService/ListPageVersions -d @)"
jq -e '([.versions[].number] | sort) == [1, 2]' <<<"${versions_json}" >/dev/null

snapshot_json="$(printf '{"siteId":"%s","name":"WSL smoke snapshot"}' "${site_id}" \
  | grpc liapoldus.management.v1.ManagementService/CreateSnapshot -d @)"
jq -e '.snapshot.pages | any(.version == 2)' <<<"${snapshot_json}" >/dev/null

printf 'Smoke test passed: site=%s page=%s, component tree, versions and snapshot.\n' "${site_id}" "${page_id}"
