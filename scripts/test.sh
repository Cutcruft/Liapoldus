#!/usr/bin/env bash
set -euo pipefail

base_url="${LIAPOLDUS_BASE_URL:-http://localhost:8080}"
suffix="$(date +%s)"
curl_args=(--noproxy '*')

echo "Checking ${base_url}"

health="$(curl "${curl_args[@]}" -fsS "${base_url}/healthz")"
grep -q '"status":"ok"' <<<"${health}"

site_json="$(curl "${curl_args[@]}" -fsS -X POST "${base_url}/api/sites" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke site\",\"slug\":\"smoke-${suffix}\"}")"
site_id="$(sed -n 's/^{"id":"\([^"]*\)".*/\1/p' <<<"${site_json}")"
test -n "${site_id}"

page_json="$(curl "${curl_args[@]}" -fsS -X POST "${base_url}/api/sites/${site_id}/pages" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Home","slug":"home","root":{"id":"root","type":"Container","children":[{"id":"title","type":"Text","props":{"text":"Hello Liapoldus"}}]}}')"
page_id="$(sed -n 's/^{"id":"\([^"]*\)".*/\1/p' <<<"${page_json}")"
test -n "${page_id}"
grep -q '"version":1' <<<"${page_json}"

updated_json="$(curl "${curl_args[@]}" -fsS -X PUT "${base_url}/api/pages/${page_id}/tree" \
  -H 'Content-Type: application/json' \
  -d '{"root":{"id":"root","type":"Container","children":[]}}')"
grep -q '"version":2' <<<"${updated_json}"

versions_json="$(curl "${curl_args[@]}" -fsS "${base_url}/api/pages/${page_id}/versions")"
grep -q '"number":1' <<<"${versions_json}"
grep -q '"number":2' <<<"${versions_json}"

snapshot_json="$(curl "${curl_args[@]}" -fsS -X POST "${base_url}/api/sites/${site_id}/snapshots" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke snapshot"}')"
grep -q '"version":2' <<<"${snapshot_json}"

echo "Smoke test passed: site, page, component tree, page versions and snapshot."
