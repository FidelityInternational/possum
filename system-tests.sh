#!/bin/sh

begin() {
  echo "----------------------Begin System Tests----------------------"
  echo "APP1_HOSTNAME: ${APP1_HOSTNAME}"
  echo "APP2_HOSTNAME: ${APP2_HOSTNAME}"
}

end() {
  echo "------------------------End System Tests------------------------"
}

fail() {
  printf " ... FAIL: %s" "$1"
  end
  exit 1
}

pass() {
   printf " ... PASS\n"
}

posturl_and_check_result() {
  echo "Endpoint: ${ENDPOINT}"
  echo "Request Data: ${DATA}"
  echo curl -X POST --max-time 10 --connect-timeout 9 -sLk "https://${POSSUM_USERNAME}:${POSSUM_PASSWORD}@${APP1_HOSTNAME}""${ENDPOINT}" -d "${DATA}"
  response=$(curl -X POST --max-time 10 --connect-timeout 9 -sLk "https://${POSSUM_USERNAME}:${POSSUM_PASSWORD}@${APP1_HOSTNAME}""${ENDPOINT}" -d "${DATA}")
  echo "Response: ${response}"

  status=$(echo "${response}" | awk "${EXPECTED}")

  if [ -z "$status" ]; then
    fail "${response} did not contain ${EXPECTED}"
  else
    pass
  fi
}

begin


# Tests against v1/state endpoint
ENDPOINT=/v1/state

EXPECTED="/.+notavalidhost is not part of my passel.+/"
DATA='{"notavalidhost": "alive"}'
posturl_and_check_result

EXPECTED="/.+possum_states.+/"
DATA='{"https://'"${APP1_HOSTNAME}"'": "dead"}'
posturl_and_check_result

EXPECTED="/.+possum_states.+/"
DATA='{"https://'"${APP1_HOSTNAME}"'": "alive"}'
posturl_and_check_result

# Tests against v1/passel_state endpoint
ENDPOINT=/v1/passel_state

EXPECTED="/.+passel_states.+/"
DATA='{"possum_states":{"https://'"${APP1_HOSTNAME}"'": "alive", "https://'"${APP2_HOSTNAME}"'": "alive"}}'
posturl_and_check_result

EXPECTED="/.+passel_states.+/"
DATA='{"possum_states":{"https://'"${APP1_HOSTNAME}"'": "dead", "https://'"${APP2_HOSTNAME}"'": "alive"}}'
posturl_and_check_result

EXPECTED="/.+Would have killed all possums.+/"
DATA='{"possum_states":{"https://'"${APP1_HOSTNAME}"'": "dead", "https://'"${APP2_HOSTNAME}"'": "dead"}}'
posturl_and_check_result

EXPECTED="/.+passel_states.+/"
DATA='{"possum_states":{"https://'"${APP1_HOSTNAME}"'": "alive", "https://'"${APP2_HOSTNAME}"'": "alive"}, "force": true}'
posturl_and_check_result

end
