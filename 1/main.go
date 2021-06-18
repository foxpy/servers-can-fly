package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	_ "modernc.org/sqlite"
	"net/http"
	"strings"
)

const schema = `
create table if not exists users(
	user_id integer primary key,
	name text unique,
	password text
);
create table if not exists sessions(
	session_id integer primary key,
	user_id integer references users,
	token text unique
);`

func main() {
	db, _ := sql.Open("sqlite", "users.db")
	db.Exec(schema)
	runServer(db)
}

func runServer(db *sql.DB) {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintln(w, "Invalid method, use POST")
			return
		}
		name := r.PostFormValue("name")
		password := r.PostFormValue("password")
		if len(name) == 0 || len(password) == 0 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintln(w, "You MUST provide name AND password")
			return
		}
		register(db, name, password)
	})
	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintln(w, "Invalid method, use POST")
			return
		}
		name := r.PostFormValue("name")
		password := r.PostFormValue("password")
		if len(name) == 0 || len(password) == 0 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintln(w, "You MUST provide name AND password")
			return
		}
		fmt.Fprintln(w, auth(db, name, password))
	})
	http.HandleFunc("/deauth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintln(w, "Invalid method, use POST")
			return
		}
		token := r.PostFormValue("token")
		deauth(db, token)
	})
	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintln(w, "Invalid method, use POST")
			return
		}
		token := r.PostFormValue("token")
		fmt.Fprintln(w, getProfile(db, token))
	})
	http.ListenAndServe(":8080", nil)
}

func register(db *sql.DB, name string, password string) {
	db.Query(`insert into users(name, password) values(?1, ?2);`, name, password)
}

func genToken(db *sql.DB, name string) string {
	rnd := make([]byte, 32)
	rand.Read(rnd)
	token := hex.EncodeToString(rnd)
	db.Exec(`insert into sessions(user_id, token) values((select user_id from users where name = ?1), ?2);`, name, token)
	return token
}

func getProfile(db *sql.DB, token string) string {
	r := db.QueryRow(`select name, password from (select name, password, token from users join sessions) where token = ?;`, token)
	var name string
	var password string
	if err := r.Scan(&name, &password); err != nil {
		return "You are not authorized"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Your name is %s and your password is %s\n", name, password)
	return b.String()
}

func auth(db *sql.DB, name string, password string) string {
	r := db.QueryRow(`select password from users where name = ?1;`, name)
	var actualPassword string
	if err := r.Scan(&actualPassword); err != nil {
		return "You are not registered"
	}
	if password != actualPassword {
		return "Invalid password"
	}
	return genToken(db, name)
}

func deauth(db *sql.DB, token string) {
	db.Exec(`delete from sessions where token = ?1;`, token)
}
