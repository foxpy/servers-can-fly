# TODO

- [x] `POST /register(name, password)`
- [x] `POST /auth(name, password) → Set-Cookie: $token`
- [x] `GET /deauth ← Cookie: $token`
- [x] `GET /profile ← Cookie: $token → $profile_info`

## Known issues

- Server always listens on fixed ip:port
- Database path is hardcoded
- Access tokens never expire automatically
- Passwords are stored in plaintext
- Some HTTP error codes used violate standard (I don't send required HTTP Headers)
- test written in Bash does not play well with locked database and sometimes fails :=DDD
- And millions of other issues, of course
