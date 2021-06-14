package main

import(
	"fmt"
	"net/http"
	"database/sql"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "users.db")
	db.Exec(`
create table if not exists users(
	name text uniqie,
	password text
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
	fmt.Println(name, password)
	db.Query(`insert into users values(?1, ?2)`, name, password)
}
