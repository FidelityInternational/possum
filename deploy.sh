#! /bin/bash

set -e

trap "rm manifest-green.yml manifest.yml" EXIT

cat > manifest-green.yml << EOF
---
applications:
- name: ${POSSUM_APP_NAME:-possum}-green
  host: ${POSSUM_APP_NAME:-possum}
  memory: 20M
  disk_quota: 50M
  instances: 2
  path: .
  services:
  - possum-db
  - possum
  env:
    GOVERSION: go1.8
    GOPACKAGENAME: github.com/FidelityInternational/possum
EOF

if [ -n "${CORS_ALLOWED}" ]; then
  echo "  env:" >> manifest-green.yml
  echo "    CORS_ALLOWED: '${CORS_ALLOWED}'" >> manifest-green.yml
fi


cat > manifest.yml << EOF
---
applications:
- name: ${POSSUM_APP_NAME:-possum}
  memory: 20M
  disk_quota: 50M
  instances: 2
  path: .
  services:
  - possum-db
  - possum
  env:
    GOVERSION: go1.8
    GOPACKAGENAME: github.com/FidelityInternational/possum
EOF
if [ -n "$CORS_ALLOWED" ]; then
  echo "  env:" >> manifest.yml
  echo "    CORS_ALLOWED: '${CORS_ALLOWED}'" >> manifest.yml
fi

: "${GLOBAL_DOMAIN:=unset}"

echo "Logging into CF..."
cf api "https://api.$CF_SYS_DOMAIN" --skip-ssl-validation
cf auth "$CF_DEPLOY_USERNAME" "$CF_DEPLOY_PASSWORD"
echo "Creating Org ${POSSUM_ORG:-possum}..."
cf create-org "${POSSUM_ORG:-possum}"
echo "Targeting Org ${POSSUM_ORG:-possum}..."
cf target -o "${POSSUM_ORG:-possum}"
echo "Creating Space ${POSSUM_SPACE:-possum}..."
cf create-space "${POSSUM_SPACE:-possum}"
echo "Targeting Space ${POSSUM_SPACE:-possum}..."
cf target -s "${POSSUM_SPACE:-possum}"
if [[ $# -gt 0 && $1 != "--managed-db" ]]; then
  echo "Setting up services..."
  if [[ "$(cf service possum-db || true)" == *"FAILED"* ]] ; then
    echo "Creating service possum-db..."
    cf cups possum-db -p "{\"database\":\"$DB_NAME\",\"host\":\"$DB_HOST\",\"port\":\"$DB_PORT\",\"username\":\"$DB_USERNAME\",\"password\":\"$DB_PASSWORD\"}"
  else
    echo "possum-db already exists..."
    echo "updating possum-db..."
    cf unbind-service "${POSSUM_APP_NAME:-possum}" possum-db
    cf uups possum-db -p "{\"database\":\"$DB_NAME\",\"host\":\"$DB_HOST\",\"port\":\"$DB_PORT\",\"username\":\"$DB_USERNAME\",\"password\":\"$DB_PASSWORD\"}"
  fi
fi
if [[ "$(cf service possum || true)" == *"FAILED"* ]] ; then
  echo "Creating service possum..."
  cf cups possum -p "{\"passel\":$PASSEL,\"username\":\"$POSSUM_USERNAME\",\"password\":\"$POSSUM_PASSWORD\"}"
else
  echo "possum already exists..."
  echo "updating possum..."
  cf unbind-service "${POSSUM_APP_NAME:-possum}" possum
  cf uups possum -p "{\"passel\":$PASSEL,\"username\":\"$POSSUM_USERNAME\",\"password\":\"$POSSUM_PASSWORD\"}"
fi
echo "Deploying apps..."
if [[ "$(cf app "${POSSUM_APP_NAME:-possum}" || true)" == *"FAILED"* ]] ; then
  cf push
  if [ "$GLOBAL_DOMAIN" != "unset" ]; then
    cf map-route "${POSSUM_APP_NAME:-possum}" "$GLOBAL_DOMAIN" -n "${POSSUM_APP_NAME:-possum}"
  fi
else
  echo "Zero downtime deploying possum..."
  cf push -f manifest-green.yml
  if [ "$GLOBAL_DOMAIN" != "unset" ]; then
    cf map-route "${POSSUM_APP_NAME:-possum}"-green "$GLOBAL_DOMAIN" -n "${POSSUM_APP_NAME:-possum}"
  fi
  cf delete "${POSSUM_APP_NAME:-possum}" -f
  cf rename "${POSSUM_APP_NAME:-possum}"-green "${POSSUM_APP_NAME:-possum}"
fi
