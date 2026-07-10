#!/bin/sh
# End-to-end smoke test against a running hush container on localhost:4874.
# Walks the README quickstart: bootstrap, secrets, agent scoping, readonly
# enforcement, rotation, and the audit trail. Exits non-zero on any failure.
set -eu

BASE=${BASE:-http://localhost:4874}
PASS=0
FAIL=0

check() { # description expected actual
  if [ "$2" = "$3" ]; then
    PASS=$((PASS + 1))
    printf '  ok   %s\n' "$1"
  else
    FAIL=$((FAIL + 1))
    printf '  FAIL %s (expected %s, got %s)\n' "$1" "$2" "$3"
  fi
}

code() { # method path [cred] [body]  -> prints HTTP status
  m=$1; p=$2; cred=${3:-}; body=${4:-}
  set -- -s -o /dev/null -w '%{http_code}' -X "$m" "$BASE$p"
  [ -n "$cred" ] && set -- "$@" -H "$cred"
  [ -n "$body" ] && set -- "$@" -H 'Content-Type: application/json' -d "$body"
  curl "$@"
}

echo "waiting for hush..."
until [ "$(code GET /healthz)" = "200" ]; do sleep 1; done

ADMIN_PW=$(docker logs hush 2>&1 | sed -n 's/.*password: //p' | tail -1)
[ -n "$ADMIN_PW" ] || { echo "could not read admin password from logs"; exit 1; }

echo "auth:"
check "healthz public" 200 "$(code GET /healthz)"
check "secrets need auth" 401 "$(code GET /api/v1/secrets/x/y)"

# Log in, keep the cookie jar.
JAR=$(mktemp)
curl -s -c "$JAR" -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"admin\",\"password\":\"$ADMIN_PW\"}" >/dev/null
ac() { curl -s -b "$JAR" "$@"; }

echo "secrets:"
ac -X PUT "$BASE/api/v1/secrets/infra/dns/cf" -H 'Content-Type: application/json' \
  -d '{"value":"cf-secret"}' >/dev/null
ac -X PUT "$BASE/api/v1/secrets/infra/dns/hz" -H 'Content-Type: application/json' \
  -d '{"value":"hz-secret"}' >/dev/null
ac -X PUT "$BASE/api/v1/secrets/media/plex/tok" -H 'Content-Type: application/json' \
  -d '{"value":"plex"}' >/dev/null
VAL=$(ac "$BASE/api/v1/secrets/infra/dns/cf" | sed -n 's/.*"value":"\([^"]*\)".*/\1/p')
check "secret round-trips" "cf-secret" "$VAL"

echo "agent token folder scope:"
TOK=$(ac -X POST "$BASE/api/v1/tokens" -H 'Content-Type: application/json' \
  -d '{"name":"smoke-agent","type":"agent","path":"infra/dns"}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
AH="Authorization: Bearer $TOK"
check "agent reads inside its folder"  200 "$(code GET /api/v1/secrets/infra/dns/cf "$AH")"
check "agent reads folder sibling"     200 "$(code GET /api/v1/secrets/infra/dns/hz "$AH")"
check "agent denied outside folder"    404 "$(code GET /api/v1/secrets/media/plex/tok "$AH")"
check "agent cannot write"             403 "$(code PUT /api/v1/secrets/infra/dns/cf "$AH" '{"value":"x"}')"
check "agent cannot browse"            403 "$(code GET /api/v1/tree/ "$AH")"

echo "readonly enforcement:"
ac -X POST "$BASE/api/v1/users" -H 'Content-Type: application/json' \
  -d '{"username":"smoke-ro","password":"smoke-ro-pass","role":"readonly"}' >/dev/null
ac -X POST "$BASE/api/v1/users/smoke-ro/grants" -H 'Content-Type: application/json' \
  -d '{"path":"infra"}' >/dev/null
RJAR=$(mktemp)
curl -s -c "$RJAR" -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"smoke-ro","password":"smoke-ro-pass"}' >/dev/null
RC="Cookie: $(sed -n 's/.*hush_session\t//p' "$RJAR" | tail -1)"
rc() { curl -s -o /dev/null -w '%{http_code}' -b "$RJAR" "$@"; }
check "readonly reads granted"    200 "$(rc "$BASE/api/v1/secrets/infra/dns/cf")"
check "readonly denied ungranted" 404 "$(rc "$BASE/api/v1/secrets/media/plex/tok")"
check "readonly cannot write"     403 "$(rc -X PUT "$BASE/api/v1/secrets/infra/dns/new" -H 'Content-Type: application/json' -d '{"value":"x"}')"

echo "rotation:"
ac -X PATCH "$BASE/api/v1/secrets/infra/dns/cf" -H 'Content-Type: application/json' \
  -d '{"rotation":{"length":16,"charset":"hex"}}' >/dev/null
V1=$(ac "$BASE/api/v1/secrets/infra/dns/cf" | sed -n 's/.*"value":"\([^"]*\)".*/\1/p')
ac -X POST "$BASE/api/v1/rotate/infra/dns/cf" >/dev/null
V2=$(ac "$BASE/api/v1/secrets/infra/dns/cf" | sed -n 's/.*"value":"\([^"]*\)".*/\1/p')
if [ "$V1" != "$V2" ] && [ ${#V2} -eq 16 ]; then
  PASS=$((PASS + 1)); echo "  ok   rotation produced a new 16-char value"
else
  FAIL=$((FAIL + 1)); echo "  FAIL rotation ($V1 -> $V2)"
fi

echo "audit:"
AUDIT=$(ac "$BASE/api/v1/audit?limit=100")
for want in secret.read secret.write secret.rotate login user.create grant.add; do
  case "$AUDIT" in
    *"$want"*) PASS=$((PASS + 1)); echo "  ok   audit has $want" ;;
    *) FAIL=$((FAIL + 1)); echo "  FAIL audit missing $want" ;;
  esac
done

rm -f "$JAR" "$RJAR"
echo
echo "passed $PASS, failed $FAIL"
[ "$FAIL" -eq 0 ]
