#!/bin/bash
# Envía los fixtures de prueba a un servidor probakgo en ejecución.
#
# Uso:
#   bash testdata/seed.sh [API_URL] [PVE_API_KEY] [PBS_API_KEY]
#
# Si se omiten los argumentos, los lee de .env en el directorio del proyecto.
# Cada key debe ser una clave pbk- activa creada para el hostname del fixture.
#
# Ejemplo:
#   bash testdata/seed.sh http://localhost:36748 pbk-clave-pve pbk-clave-pbs

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

API_URL="${1:-}"
PVE_API_KEY="${2:-}"
PBS_API_KEY="${3:-}"

# Cargar desde .env si no se pasaron como argumentos
if [ -z "$API_URL" ] || [ -z "$PVE_API_KEY" ] || [ -z "$PBS_API_KEY" ]; then
    ENV_FILE="$PROJECT_ROOT/.env"
    if [ -f "$ENV_FILE" ]; then
        # shellcheck disable=SC1090
        source <(grep -v '^#' "$ENV_FILE" | grep -v '^$')
    fi
    API_URL="${API_URL:-http://localhost:36748}"
    PVE_API_KEY="${PVE_API_KEY:-}"
    PBS_API_KEY="${PBS_API_KEY:-}"
fi

if [ -z "$PVE_API_KEY" ] || [ -z "$PBS_API_KEY" ]; then
    echo "ERROR: Se necesitan dos API keys pbk-, una para soporte1 y otra para pbs-test."
    echo "Uso: bash testdata/seed.sh [URL] [PVE_API_KEY] [PBS_API_KEY]"
    echo "  o: añade PVE_API_KEY=pbk-... y PBS_API_KEY=pbk-... a .env"
    exit 1
fi

for key_name in PVE_API_KEY PBS_API_KEY; do
    key="${!key_name}"
    if [[ "$key" != pbk-* ]]; then
        echo "ERROR: $key_name debe empezar por pbk-."
        exit 1
    fi
done

send() {
    local label="$1"
    local endpoint="$2"
    local file="$3"
    local api_key="$4"
    local machine_id="$5"

    printf "%-20s → %s ... " "$label" "$endpoint"

    response=$(curl -sS -w "\n%{http_code}" \
        -X POST "$API_URL/api/$endpoint" \
        -H "Authorization: Bearer $api_key" \
        -H "X-Machine-ID: $machine_id" \
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
echo "PVE key  : ${PVE_API_KEY:0:12}..."
echo "PBS key  : ${PBS_API_KEY:0:12}..."
echo ""

send "PVE (soporte1)" "report/pve" "$SCRIPT_DIR/fixture_pve.json" \
    "$PVE_API_KEY" "11223344-5566-7788-99aa-bbccddeeff00"
send "PBS (pbs-test)" "report/pbs" "$SCRIPT_DIR/fixture_pbs.json" \
    "$PBS_API_KEY" "aabbccdd-eeff-0011-2233-445566778899"

echo ""
echo "Listo. Abre $API_URL para verificar en el dashboard."
