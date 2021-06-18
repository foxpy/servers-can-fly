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
- And millions of other issues, of course
