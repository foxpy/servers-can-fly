package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"net/http"
	"strings"
)

func main() {
	db, _ := sql.Open("sqlite", "users.db")
	db.Exec(`
create table if not exists users(
	user_id integer primary key,
	name text unique,
	password text
);
create table if not exists sessions(
	session_id integer primary key,
	user_id integer references users,
	token text unique
);
`)
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		name := r.PostFormValue("name")
		password := r.PostFormValue("password")
		if len(name) == 0 || len(password) == 0 {
			return
		}
		register(db, name, password)
	})
	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		name := r.PostFormValue("name")
		password := r.PostFormValue("password")
		token := auth(db, name, password)
		fmt.Fprintf(w, "%s", token)
	})
	http.HandleFunc("/deauth", func(w http.ResponseWriter, r *http.Request) {
		token := r.PostFormValue("token")
		deauth(db, token)
	})
	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		token := r.PostFormValue("token")
		profile := getProfile(db, token)
		fmt.Fprintf(w, "%s", profile)
	})
	http.ListenAndServe(":8080", nil)
}

func register(db *sql.DB, name string, password string) {
	db.Query(`insert into users(name, password) values(?1, ?2)`, name, password)
}

func genToken(db *sql.DB, name string) string {
	var b strings.Builder
	r := make([]byte, 32)
	rand.Read(r)
	for i := 0; i < 32; i++ {
		fmt.Fprintf(&b, "%02x", r[i])
	}
	token := b.String()
	db.Exec(`insert into sessions(user_id, token) values((select user_id from users where name = ?1), ?2)`, name, token)
	return token
}

func getProfile(db *sql.DB, token string) string {
	r := db.QueryRow(`select name, password from (select name, password, token from users join sessions) where token = ?`, token)
	var name string
	var password string
	if err := r.Scan(&name, &password); err != nil {
		return "You are not authorized"
	} else {
		var b strings.Builder
		fmt.Fprintf(&b, "Your name is %s and your password is %s", name, password)
		return b.String()
	}
}

func auth(db *sql.DB, name string, password string) string {
	r := db.QueryRow(`select password from users where name = ?1`, name)
	var actualPassword string
	if err := r.Scan(&actualPassword); err != nil {
		return "You are not registered"
	} else if password == actualPassword {
		return genToken(db, name)
	} else {
		return "Invalid password"
	}
}

func deauth(db *sql.DB, token string) {
	db.Exec(`delete from sessions where token = ?1`, token)
}
