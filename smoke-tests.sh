#!/bin/sh

begin() {
  echo "----------------------Begin Smoke Tests----------------------"
  echo "${APP_URL}"
}

end() {
  echo "------------------------End Smoke Tests------------------------"
}

fail() {
  printf " ... FAIL: %s" "$1"
  end
  exit 1
}

pass() {
   printf " ... PASS\n"
}

geturl_and_check_result() {
  response=$(curl --max-time 10 --connect-timeout 3 -svLk "${APP_URL}""${ENDPOINT}")
  echo "Response: ${response}"

  status=$(echo "${response}" | awk "${EXPECTED}")

  if [ -z "$status" ]; then
    fail "${response} did not contain ${EXPECTED}"
  else
    pass
  fi
}

begin

ENDPOINT=/v1/state
EXPECTED="/.+state.+/"
geturl_and_check_result

ENDPOINT=/v1/passel_state
EXPECTED="/.+possum_states.+/"
geturl_and_check_result

end
