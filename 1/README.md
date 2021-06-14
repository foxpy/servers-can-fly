# TODO

- [x] `POST /register(name, password)`
- [x] `POST /auth(name, password) -> token`
- [x] `POST /deauth(token)`
- [x] `POST /profile(token) -> "profile info"`

## Known issues

- Server always listens on fixed ip:port
- Server does not perform any error checking
- API never returns meaningfull errors, everything is always in human readable text only
- Database path is hardcoded
- Access tokens never expire automatically
- Passwords are stored in plaintext
- And billions of other issues, of course
