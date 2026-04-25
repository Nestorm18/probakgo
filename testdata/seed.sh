#!/bin/bash
# Envía los fixtures de prueba a un servidor probakgo en ejecución.
#
# Uso:
#   bash testdata/seed.sh [API_URL] [API_KEY]
#
# Si se omiten los argumentos, los lee de .env en el directorio del proyecto.
# API_KEY debe ser una clave pbk- activa creada desde la interfaz web.
#
# Ejemplo:
#   bash testdata/seed.sh http://localhost:36748 pbk-tuclaveaqui

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

API_URL="${1:-}"
API_KEY="${2:-}"

# Cargar desde .env si no se pasaron como argumentos
if [ -z "$API_URL" ] || [ -z "$API_KEY" ]; then
    ENV_FILE="$PROJECT_ROOT/.env"
    if [ -f "$ENV_FILE" ]; then
        # shellcheck disable=SC1090
        source <(grep -v '^#' "$ENV_FILE" | grep -v '^$')
    fi
    API_URL="${API_URL:-http://localhost:36748}"
    API_KEY="${API_KEY:-}"
fi

if [ -z "$API_KEY" ]; then
    echo "ERROR: Se necesita una API key pbk-."
    echo "Uso: bash testdata/seed.sh [URL] [API_KEY]"
    echo "  o: añade API_KEY=pbk-... a .env"
    exit 1
fi

if [[ "$API_KEY" != pbk-* ]]; then
    echo "ERROR: La key debe empezar por pbk-  (recibido: ${API_KEY:0:8}...)"
    exit 1
fi

send() {
    local label="$1"
    local endpoint="$2"
    local file="$3"

    printf "%-20s → %s ... " "$label" "$endpoint"

    response=$(curl -s -w "\n%{http_code}" \
        -X POST "$API_URL/api/$endpoint" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        --data-binary "@$file")

    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | head -n -1)

    if [ "$http_code" = "200" ]; then
        echo "OK"
    else
        echo "FAIL (HTTP $http_code)"
        echo "  Respuesta: $body"
        exit 1
    fi
}

echo "Servidor : $API_URL"
echo "API key  : ${API_KEY:0:12}..."
echo ""

send "PBS (pbs-test)"    "report/pbs" "$SCRIPT_DIR/fixture_pbs.json"
send "PVE (soporte1)"    "report/pve" "$SCRIPT_DIR/fixture_pve.json"

echo ""
echo "Listo. Abre $API_URL para verificar en el dashboard."
