package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
	"errors"
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
	db, err := sql.Open("sqlite", "users.db")
	if err != nil {
		log.Fatalln("Failed to open database file", err.Error())
	}
	_, err = db.Exec(schema)
	if err != nil {
		log.Fatalln("Failed to apply database schema", err.Error())
	}
	runServer(db)
}

func runServer(db *sql.DB) {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) { registerHandler(db, w, r) })
	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) { authHandler(db, w, r) })
	http.HandleFunc("/deauth", func(w http.ResponseWriter, r *http.Request) { deauthHandler(db, w, r) })
	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) { profileHandler(db, w, r) })
	http.ListenAndServe(":8080", nil)
}

func registerHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
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
	_, err := db.Exec(`insert into users(name, password) values(?1, ?2);`, name, password)
	if err != nil {
		// there should be a clean way to check if user is already registered
		// a naive approach (select, check and then register) is not a transaction,
		// so it is prune to race conditions, even in this simple application
		// without even a functionality to delete your account
		// I should research SQL transactions and SQL error code returning
		// but it probably falls out of topic of this assignment
		// HTTP 500 seems to be good enough for now
		log.Printf("Failed to register user '%s' with password '%s': %s\n", name, password, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func authHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
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
	token, err := auth(db, name, password)
	if err != nil {
		log.Println("Failed to authorize user", name, ":", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Set-Cookie", token)
}

func deauthHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Invalid method, use GET")
		return
	}
	token, err := tokenFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, err.Error())
		return
	}
	_, err = db.Exec(`delete from sessions where token = ?1;`, token)
	// at this point, user has no idea if his token was valid before his
	// request to deauthorization, but at least he can be sure that supplied
	// token is no longer valid after receiving HTTP 200 query reply
	if err != nil {
		log.Printf("Failed to delete access token '%s' from database: %s\n", token, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func profileHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Invalid method, use GET")
		return
	}
	token, err := tokenFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, err.Error())
		return
	}
	var name string
	var password string
	row := db.QueryRow(`select name, password from (select * from users join sessions) where token = ?;`, token)
	if err := row.Scan(&name, &password); err != nil {
		log.Println("Failed to query user data from database", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Your name is %s and your password is %s\n", name, password)
}

func tokenFromRequest(r *http.Request) (string, error) {
	// here I assume that r.Header values (arrays of strings) always have at least one element,
	// documentation does not tell anything about it: https://golang.org/pkg/net/http/#Header
	cookie, exists := r.Header["Cookie"]
	if !exists {
		return "", errors.New("Cookie with access token must be supplied")
	}
	var token string
	fmt.Sscanf(cookie[0], "sessionToken=%s", &token)
	if len(token) != 64 {
		return "", errors.New("Token length is invalid")
	}
	return token, nil
}

func genTokenAndStore(db *sql.DB, name string) (string, error) {
	rnd := make([]byte, 32)
	_, err := rand.Read(rnd)
	if err != nil {
		return "", err
	}
	token := hex.EncodeToString(rnd)
	_, err = db.Exec(`insert into sessions(user_id, token) values((select user_id from users where name = ?1), ?2);`, name, token)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sessionToken=%s", token), nil
}

func auth(db *sql.DB, name string, password string) (string, error) {
	r := db.QueryRow(`select password from users where name = ?1;`, name)
	var actualPassword string
	err := r.Scan(&actualPassword)
	if err != nil {
		return "", err
	}
	if password != actualPassword {
		return "Invalid password", nil
	}
	token, err := genTokenAndStore(db, name)
	if err != nil {
		return "", err
	}
	return token, nil
}
