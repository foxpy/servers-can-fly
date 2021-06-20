package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
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
	err := http.ListenAndServe(":8080", nil)
	if err != http.ErrServerClosed {
		log.Fatalf("Failed to run HTTP server: %s", err.Error())
	}
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
	isAlreadyRegistered, err := isUserRegistered(db, name)
	if err != nil {
		log.Printf("Failed to check if user '%s' is registered in database: %s", name, err.Error())
		// at this point, we can't really tell anything useful to users
		// so we just let them enjoy classic HTTP 500
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if isAlreadyRegistered {
		// I __really__ don't know a good HTTP error code for this case
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintln(w, "You are already registered")
		return
	}
	// there is a possible (yet harmless, because of database UNIQUE constraint) data race:
	// user may register during processing of this request
	_, err = db.Exec(`insert into users(name, password) values(?1, ?2);`, name, password)
	if err != nil {
		log.Printf("Failed to register user '%s' with password '%s': %s", name, password, err.Error())
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
	isRegistered, err := isUserRegistered(db, name)
	if err != nil {
		log.Printf("Failed to check if user '%s' is registered in database: %s", name, err.Error())
		// at this point, we can't really tell anything useful to users
		// so we just let them enjoy classic HTTP 500
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !isRegistered {
		// I couldn't pick a proper error code for this one
		// There is probably a better choice which I do not know about
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Println(w, "You are not registered")
		return
	}
	// from this point, there is a possible harmless data race
	// if user deletes their account during processing of this request.
	// however, this application does not implement account
	// deletion (at least for now), so this race will never happen
	actualPassword, err := getUserPassword(db, name)
	if err != nil {
		log.Printf("Failed to get user '%s' password from database: %s", name, err.Error())
		// at this point, we can't really tell anything useful to users
		// so we just let them enjoy classic HTTP 500
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if password != actualPassword {
		// I couldn't pick a proper error code for this one
		// There is probably a better choice which I do not know about
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintln(w, "Invalid password")
		return
	}
	token, err := genTokenAndStore(db, name)
	if err != nil {
		log.Println("Failed to generate access token for user", name, ":", err.Error())
		// at this point, we can't return anything fancier than just plain HTTP 500
		// I don't think piping all internal errors is a good idea
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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
		log.Printf("Failed to delete access token '%s' from database: %s", token, err.Error())
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

func getUserPassword(db *sql.DB, name string) (string, error) {
	row := db.QueryRow(`select password from users where name = ?1;`, name)
	var password string
	err := row.Scan(&password)
	if err != nil {
		return "", err
	}
	return password, nil
}

func isUserRegistered(db *sql.DB, name string) (bool, error) {
	row := db.QueryRow(`select count(user_id) from users where name = ?1;`, name)
	var cnt uint64
	err := row.Scan(&cnt)
	if err != nil {
		return false, err
	}
	if cnt > 1 {
		// this should never happen because of UNIQUE constraint
		return false, errors.New(fmt.Sprintln("Database UNIQUE constraint is violated, there is", cnt, "users with name", name))
	}
	return cnt == 1, nil
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
