#!/bin/bash

: "${PORTAINER_URL:?PORTAINER_URL tidak di-set}"
: "${PORTAINER_USERNAME:?PORTAINER_USERNAME tidak di-set}"
: "${PORTAINER_PASSWORD:?PORTAINER_PASSWORD tidak di-set}"
: "${STACK_NAME:?STACK_NAME tidak di-set}"

# Timeout total: MAX_RETRY * SLEEP_INTERVAL detik
MAX_RETRY=60   # 60 × 10s = 10 menit
SLEEP_INTERVAL=10

echo "🔐 Autentikasi ke Portainer..."
TOKEN=$(curl -s -X POST "https://${PORTAINER_URL}/api/auth" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"${PORTAINER_USERNAME}\", \"password\": \"${PORTAINER_PASSWORD}\"}" \
  | jq -r .jwt)

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "❌ Autentikasi gagal! Cek PORTAINER_URL, USERNAME, dan PASSWORD."
  exit 1
fi

echo "🔍 Mencari stack: $STACK_NAME..."
STACK=$(curl -s -X GET "https://${PORTAINER_URL}/api/stacks" \
  -H "Authorization: Bearer ${TOKEN}" \
  | jq ".[] | select(.Name == \"$STACK_NAME\")")

if [ -z "$STACK" ]; then
  echo "❌ Stack '$STACK_NAME' tidak ditemukan di Portainer!"
  exit 1
fi

STACK_ID=$(echo "$STACK" | jq -r .Id)
ENDPOINT_ID=$(echo "$STACK" | jq -r .EndpointId)
ENV=$(echo "$STACK" | jq '.Env // []')

# ── Catat container ID lama sebelum redeploy ──────────────────────────────────
echo "📸 Mencatat container aktif sebelum redeploy..."
CONTAINERS_BEFORE=$(curl -s -X GET \
  "https://${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/json?all=true&filters=%7B%22label%22%3A%5B%22com.docker.compose.project%3D${STACK_NAME}%22%5D%7D" \
  -H "Authorization: Bearer ${TOKEN}")

OLD_IDS=$(echo "$CONTAINERS_BEFORE" | jq -r '[.[] | .Id] | join(",")')
echo "   Container lama: $(echo "$CONTAINERS_BEFORE" | jq -r '[.[] | .Names[0]] | join(", ")')"

# ── Ambil compose file lalu trigger redeploy ─────────────────────────────────
echo "📄 Mengambil compose file..."
STACK_FILE=$(curl -s -X GET "https://${PORTAINER_URL}/api/stacks/${STACK_ID}/file" \
  -H "Authorization: Bearer ${TOKEN}" \
  | jq -r .StackFileContent)

PAYLOAD=$(jq -n \
  --arg content "$STACK_FILE" \
  --argjson env "$ENV" \
  '{stackFileContent: $content, env: $env, pullImage: true}')

echo "🚀 Triggering redeploy $STACK_NAME (pull latest image)..."
HTTP_STATUS=$(curl -s -o /tmp/portainer_response.json -w "%{http_code}" \
  -X PUT "https://${PORTAINER_URL}/api/stacks/${STACK_ID}?endpointId=${ENDPOINT_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD")

if [ "$HTTP_STATUS" != "200" ]; then
  echo "❌ Redeploy gagal! HTTP Status: $HTTP_STATUS"
  cat /tmp/portainer_response.json | jq .
  exit 1
fi

echo "⏳ Menunggu image selesai di-pull dan container baru running..."
echo "   (Timeout: $((MAX_RETRY * SLEEP_INTERVAL)) detik)"

COUNT=0
while [ $COUNT -lt $MAX_RETRY ]; do
  sleep $SLEEP_INTERVAL
  COUNT=$((COUNT + 1))

  CONTAINERS=$(curl -s -X GET \
    "https://${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/json?all=true&filters=%7B%22label%22%3A%5B%22com.docker.compose.project%3D${STACK_NAME}%22%5D%7D" \
    -H "Authorization: Bearer ${TOKEN}")

  # Container baru = ID tidak ada di daftar container lama
  NEW_RUNNING=$(echo "$CONTAINERS" | jq \
    --arg old "$OLD_IDS" \
    '[.[] | select(.State == "running" and ((.Id) as $id | ($old | split(",") | index($id)) == null))] | length')

  FAILED=$(echo "$CONTAINERS" | jq \
    '[.[] | select(.State == "exited" and (.Status | test("Exited \\(0\\)") | not) and (.Names[0] | test("seed") | not))] | length')

  echo "🔄 [$((COUNT * SLEEP_INTERVAL))s / $((MAX_RETRY * SLEEP_INTERVAL))s] Container baru running: ${NEW_RUNNING} | Gagal: ${FAILED}"
  echo "$CONTAINERS" | jq -r '.[] | "   → \(.Names[0]) | \(.State) | \(.Status) | id: \(.Id[:12])"'

  if [ "$FAILED" -gt "0" ]; then
    echo ""
    echo "❌ Ada container yang crash!"
    echo "$CONTAINERS" | jq -r '.[] | select(.State == "exited" and (.Status | test("Exited \\(0\\)") | not) and (.Names[0] | test("seed") | not)) | "   → \(.Names[0]) | \(.Status)"'
    exit 1
  fi

  if [ "$NEW_RUNNING" -gt "0" ]; then    
    # Cleanup dangling images setelah redeploy sukses
    echo "🧹 Membersihkan dangling images..."
    curl -s -X POST "https://${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/images/prune" \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "Content-Type: application/json" \
      -d '{"filters":{"dangling":["true"]}}' | jq -r '"   Reclaimed: \(.SpaceReclaimed // 0 | . / 1073741824 | tostring | .[0:5]) GB"'
    
    echo "✅ Cleanup selesai!"
    echo ""
    echo "✅ Stack $STACK_NAME berhasil di-redeploy dengan image baru dan running!"
    exit 0
  fi

  
done

echo ""
echo "❌ Timeout $((MAX_RETRY * SLEEP_INTERVAL))s! Container baru tidak kunjung running."
echo "   Kemungkinan image masih dalam proses pull atau ada error di server."
exit 1
