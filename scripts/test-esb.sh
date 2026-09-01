#!/usr/bin/env bash
set -euo pipefail

grpcurl_bin="${GRPCURL_BIN:-grpcurl}"
address="${LIAPOLDUS_GRPC_ADDR:-127.0.0.1:8080}"
proto_root="${LIAPOLDUS_PROTO_ROOT:-proto}"

set +e
response="$("${grpcurl_bin}" -plaintext \
  -import-path "${proto_root}" \
  -proto esb/esb.proto \
  -d '{"payload":"","metadata":{"service":"missing","method":"missing","content_type":"application/json"}}' \
  "${address}" esb.Esb/Call 2>&1)"
status=$?
set -e

test "${status}" -ne 0
grep -q 'Code: NotFound' <<<"${response}"
echo "ESB smoke test passed: unknown operation is mapped to gRPC NotFound."
