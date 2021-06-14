package main

import(
	"net/http"
	"database/sql"
	_ "modernc.org/sqlite"
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
	http.ListenAndServe(":8080", nil)
}

func register(db *sql.DB, name string, password string) {
	db.Query(`insert into users(name, password) values(?1, ?2)`, name, password)
}
