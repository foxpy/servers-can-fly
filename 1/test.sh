#!/bin/bash
let failures=0

curl -X POST -F "name=foxpy" -F "password=qwertz" http://localhost:8080/register &>/dev/null
db_user="$(printf "select name, password from users where name = 'foxpy'" | sqlite3 users.db)"
if [[ "$db_user" != "foxpy|qwertz" ]]; then
	let failues+=1
	echo "Test 1 failed: registered user not in database"
else
	echo "Test 1 succeeded"
fi

token="$(curl -v -F "name=foxpy" -F "password=qwertz" http://localhost:8080/auth 2>&1 | grep -oE '([A-Fa-f0-9]{64})')"
if [[ "$token" ]]; then
	echo "Test 2 succeeded"
else
	let failures+=1
	echo "Test 2 failed: no access token granted after registration"
fi

profile="$(curl -H "Cookie: $token" http://localhost:8080/profile 2>/dev/null)"
if [[ "$profile" != "Your name is foxpy and your password is qwertz" ]]; then
	let failures+=1
	echo "Test 3 failed: can't get profile after authorization"
else
	echo "Test 3 succeeded"
fi

curl -H "Cookie: $token" http://localhost:8080/deauth &>/dev/null
db_token="$(printf "select token from sessions where token = '$token'" | sqlite3 users.db)"
if [[ "$db_token" != "" ]]; then
	let failures+=1
	echo "Test 4 failed: token has not been deleted after deauthorization"
else
	echo "Test 4 succeeded"
fi

if [[ "$failures" > 0 ]]; then
	echo "$failures tests failed"
	exit 1
else
	exit 0
fi
