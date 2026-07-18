#!/usr/bin/env bash
# scripts/smoke-mvp.sh — MVP 端到端冒烟：
# 导入亚马逊榜单 CSV → 板块/分类浏览 → 商品详情/档案 → 选品提交需求（AI 预填）
# → LLM 生成多版文案 → 审批/打回 → 定稿交付导出。
# 运行方式（Git Bash）：bash scripts/smoke-mvp.sh
set -euo pipefail

PORT="${SMOKE_PORT:-18081}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DB_REL="./data/smoke-mvp.sqlite3"
DB_ABS="$ROOT/data/smoke-mvp.sqlite3"
TMP_DIR="$ROOT/scripts/.tmp"
FIXTURE_REL="scripts/fixtures/amazon_hot.csv"
BIN="$TMP_DIR/tkshop.exe"
BASE="http://127.0.0.1:${PORT}/api/v1"

mkdir -p "$TMP_DIR"
rm -f "$DB_ABS" "$DB_ABS-shm" "$DB_ABS-wal"

echo "== build tkshop =="
go build -o "$BIN" ./cmd/tkshop

echo "== start server on :$PORT (db: $DB_REL) =="
TTS_DATABASE_URL="sqlite:///$DB_REL" "$BIN" serve --addr ":$PORT" >"$TMP_DIR/server.log" 2>&1 &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null || true' EXIT

READY=0
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then READY=1; break; fi
  sleep 0.5
done
[ "$READY" = "1" ] || { echo "FAIL  server did not start"; tail -50 "$TMP_DIR/server.log"; exit 1; }

PASS=0
ok()   { PASS=$((PASS + 1)); echo "PASS  $1"; }
fail() { echo "FAIL  $1"; echo "--- server log tail ---"; tail -50 "$TMP_DIR/server.log" || true; exit 1; }
jqe()  { python -c 'import sys,json,os;d=json.load(sys.stdin);print(eval(sys.argv[1]))' "$1"; }

# --- 1. 鉴权 ---
ADMIN_LOGIN=$(curl -fsS -X POST "$BASE/auth/login" -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}') || fail "operator login"
TOKEN=$(echo "$ADMIN_LOGIN" | jqe "d['token']")
[ -n "$TOKEN" ] || fail "operator token empty"
ok "operator 登录"

OP=(-H "Authorization: Bearer $TOKEN")
JSON=(-H "Content-Type: application/json")

curl -fsS -X POST "$BASE/users" "${OP[@]}" "${JSON[@]}" \
  -d '{"username":"client1","password":"client123","role":"client","display_name":"甲方一号"}' >/dev/null || fail "create client user"
ok "创建甲方账号"

CTOKEN=$(curl -fsS -X POST "$BASE/auth/login" "${JSON[@]}" \
  -d '{"username":"client1","password":"client123"}' | jqe "d['token']")
[ -n "$CTOKEN" ] || fail "client token empty"
CL=(-H "Authorization: Bearer $CTOKEN")
ok "甲方登录"

# 单端模式：client 与 operator 等价，可访问运营路由。
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/imports" "${CL[@]}")
[ "$CODE" = "200" ] || fail "client 访问 imports 应为 200，实得 $CODE"
ok "甲方可访问 imports（单端模式）"

# --- 2. 导入亚马逊榜单 CSV ---
IMPORT_RESP=$(curl -fsS -X POST "$BASE/imports/csv" "${OP[@]}" \
  -F "file=@$FIXTURE_REL" -F "region=US" -F "idempotency_key=smoke-mvp-1") || fail "csv import"
[ "$(echo "$IMPORT_RESP" | jqe "d['imported_rows']")" = "10" ] || fail "应导入 10 行：$IMPORT_RESP"
ok "导入榜单 CSV（10 行）"

# --- 3. 板块/分类浏览 ---
MKTS=$(curl -fsS "$BASE/marketplaces" "${CL[@]}") || fail "list marketplaces"
[ "$(echo "$MKTS" | jqe "sorted(d['items'])")" = "['DE', 'JP', 'US']" ] || fail "站点列表异常：$MKTS"
ok "板块（站点）列表"

TRENDS=$(curl -fsS "$BASE/trends?marketplace=US&page_size=20" "${CL[@]}") || fail "list trends"
US_COUNT=$(echo "$TRENDS" | jqe "len(d['items'])")
[ "$US_COUNT" -ge 5 ] || fail "US 站点商品不足：$US_COUNT"
PID=$(echo "$TRENDS" | jqe "d['items'][0]['id']")
ok "按板块浏览热点商品（US $US_COUNT 个）"

# --- 4. 商品详情/档案 ---
DETAIL=$(curl -fsS "$BASE/products/$PID" "${CL[@]}") || fail "product detail"
echo "$DETAIL" | jqe "d['product']['id'] == '$PID' and d['detail'] is not None" >/dev/null || fail "详情缺少档案"
ok "商品详情与档案"

# --- 4.5 商品档案素材区 ---
ASSET_RESP=$(curl -fsS -X POST "$BASE/products/$PID/assets" "${CL[@]}" "${JSON[@]}" \
  -d '{"kind":"image","url":"https://example.com/evidence.jpg","source":"amazon.com","note":"冒烟素材"}') || fail "add dossier asset"
echo "$ASSET_RESP" | jqe "d['id'].startswith('da_') and d['kind'] == 'image' and d['created_by'] == 'client1'" >/dev/null \
  || fail "素材创建响应异常：$ASSET_RESP"
ok "商品档案添加图片素材（201）"

ASSETS=$(curl -fsS "$BASE/products/$PID/assets?kind=image" "${CL[@]}") || fail "list dossier assets"
export SMOKE_ASSET_URL="https://example.com/evidence.jpg"
[ "$(echo "$ASSETS" | jqe "any(x['url'] == os.environ['SMOKE_ASSET_URL'] and x['kind'] == 'image' for x in d['items'])")" = "True" ] \
  || fail "素材列表缺少刚添加的图片：$ASSETS"
ok "商品素材列表可查（kind=image）"

# --- 5. 选品需求（AI 预填 + 提交） ---
SUGGEST=$(curl -fsS -X POST "$BASE/products/$PID/prefill" "${CL[@]}") || fail "prefill"
U_USAGE=$(echo "$SUGGEST" | jqe "d['suggestion']['usage']")
U_STYLE=$(echo "$SUGGEST" | jqe "d['suggestion']['style']")
U_FOCUS=$(echo "$SUGGEST" | jqe "d['suggestion']['focus']")
[ -n "$U_USAGE" ] || fail "prefill 返回空 usage"
ok "AI 预填需求草稿"

REQ_RESP=$(curl -fsS -X POST "$BASE/requests" "${CL[@]}" "${JSON[@]}" \
  -d "{\"product_id\":\"$PID\",\"usage\":\"$U_USAGE\",\"style\":\"$U_STYLE\",\"focus\":\"$U_FOCUS\",\"notes\":\"冒烟测试需求\"}") || fail "create request"
RID=$(echo "$REQ_RESP" | jqe "d['id']")
[ -n "$RID" ] || fail "request id empty"
export SMOKE_RID="$RID"
ok "提交选品需求"

# --- 6. LLM 生成（单端模式：client 也可触发） ---
GEN_RESP=$(curl -fsS -X POST "$BASE/requests/$RID/generate" "${CL[@]}") || fail "generate"
[ "$(echo "$GEN_RESP" | jqe "len(d['items'])")" = "3" ] || fail "应生成 3 版文案"
VID=$(echo "$GEN_RESP" | jqe "d['items'][0]['id']")
ok "甲方生成 3 版文案（单端模式，mock provider）"

# --- 7. 审批（client）与交付（operator） ---
APP_RESP=$(curl -fsS -X POST "$BASE/requests/$RID/approve" "${CL[@]}" "${JSON[@]}" \
  -d "{\"variant_id\":\"$VID\",\"note\":\"第 1 版可以\"}") || fail "approve"
[ "$(echo "$APP_RESP" | jqe "d['request']['status']")" = "approved" ] || fail "审批后状态异常"
ok "甲方审批通过"

DLV_RESP=$(curl -fsS -X POST "$BASE/requests/$RID/deliver" "${OP[@]}" "${JSON[@]}" \
  -d "{\"variant_id\":\"$VID\"}") || fail "deliver"
[ "$(echo "$DLV_RESP" | jqe "d['request']['status']")" = "delivered" ] || fail "交付后状态异常"
ok "定稿交付"

REVIEW=$(curl -fsS "$BASE/requests/$RID/review" "${CL[@]}") || fail "review aggregate"
echo "$REVIEW" | jqe "d['request']['status'] == 'delivered' and d['delivery'] is not None and len(d['events']) >= 1" >/dev/null \
  || fail "交付聚合校验失败：$REVIEW"
ok "交付记录与审批历史可查证"

DELIVERIES=$(curl -fsS "$BASE/deliveries" "${OP[@]}") || fail "list deliveries"
export SMOKE_PID="$PID"
[ "$(echo "$DELIVERIES" | jqe "any(x['package']['product']['id'] == os.environ['SMOKE_PID'] for x in d['items'])")" = "True" ] \
  || fail "交付列表缺少对应商品"
ok "交付导出列表"

echo
echo "SMOKE OK — $PASS 项检查全部通过"
