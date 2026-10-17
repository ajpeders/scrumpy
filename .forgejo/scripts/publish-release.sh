#!/usr/bin/env bash
# Publish a tag release to Forgejo.
#
# Idempotent: looks up an existing release for the tag first and only
# creates one if none exists. For each asset, checks the release's
# existing attachments and skips any whose name matches; uploads the
# rest. This makes the job safe to re-run after a partial failure.
#
# Usage:
#   publish-release.sh <api> <token> <tag> <asset>...
#
# <api> is the repo's releases endpoint, e.g.
#   https://host/api/v1/repos/<owner>/<repo>/releases
#
# Required env:
#   JQ_BIN - path to jq (default: jq)
#
# API contract (Forgejo only; GitHub's upload API differs, so
# .github/workflows/release.yml uses the gh CLI instead):
#   GET  <api>/tags/<tag>             → 200 release JSON | 404
#   POST <api>                        body: {tag_name,name} → 201 release JSON
#   GET  <api>/<id>/assets?page=N     → 200 array of {name,...}
#   POST <api>/<id>/assets?name=N     multipart → 201 created
set -euo pipefail

api=$1
token=$2
tag=$3
shift 3
assets=("$@")

: "${JQ_BIN:=jq}"

# Look up the release by tag. We probe with a HEAD-like request to get
# the status code without consuming the body, then either reuse the body
# or create one. The two-step avoids buffering the full body when we
# only need to know whether it exists.
http=$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "Authorization: token ${token}" \
    "${api}/tags/${tag}") || true

case "$http" in
    200)
        body=$(curl -fsS -H "Authorization: token ${token}" \
            "${api}/tags/${tag}")
        id=$(printf '%s' "$body" | "$JQ_BIN" -r '.id // empty')
        ;;
    404)
        response=$(curl -fsS -X POST -H "Authorization: token ${token}" \
            -H 'Content-Type: application/json' \
            -d "{\"tag_name\":\"${tag}\",\"name\":\"${tag}\"}" \
            "${api}")
        id=$(printf '%s' "$response" | "$JQ_BIN" -r '.id // empty')
        ;;
    *)
        echo "release lookup failed: HTTP $http" >&2
        exit 1
        ;;
esac

if [ -z "$id" ] || [ "$id" = "null" ]; then
    echo "release id missing in response" >&2
    exit 1
fi

if ! printf '%s' "$id" | grep -Eq '^[0-9]+$'; then
    echo "release id is not an integer: $id" >&2
    exit 1
fi

# Collect the names of assets already on the release so we can skip
# them. Forgejo paginates this endpoint (default page size is 10);
# follow pages until we have them all.
have=""
page=1
while :; do
    response=$(curl -fsS -H "Authorization: token ${token}" \
        "${api}/${id}/assets?page=${page}&limit=50")
    n=$(printf '%s' "$response" | "$JQ_BIN" -r 'length')
    if [ "$n" = "0" ] || [ "$n" = "null" ]; then
        break
    fi
    page_names=$(printf '%s' "$response" | "$JQ_BIN" -r '.[].name')
    have="${have}${page_names}"$'\n'
    if [ "$n" -lt 50 ]; then
        break
    fi
    page=$((page + 1))
done
have=$(printf '%s' "$have" | sort -u)

for f in "${assets[@]}"; do
    # Compare by basename: callers pass full paths from the workspace,
    # Forgejo stores just the filename.
    name=$(basename "$f")
    if printf '%s\n' "$have" | grep -Fxq "$name"; then
        echo "skipping existing asset $name" >&2
        continue
    fi
    curl -fsS -X POST -H "Authorization: token ${token}" \
        -F "attachment=@${f}" "${api}/${id}/assets?name=${name}"
done