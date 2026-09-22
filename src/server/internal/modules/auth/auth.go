package auth

import (
	"fmt"
	"net/http"

	pw "social-network/internal/platform/password"
)

type Login struct {
	HashedPassword string
	SessionToken   string
	CSRFToken      string
}

var users = map[string]Login{}

func Register(w http.ResponseWriter, r *http.Request) {

	username := r.FormValue("username")
	password := r.FormValue("password")

	if len(username) < 8 || len(password) < 8 {
		er := http.StatusNotAcceptable
		http.Error(w, "Invalid username/password length", er)
		return
	}

	// replace with db
	if _, ok := users[username]; ok {
		er := http.StatusConflict
		http.Error(w, "User already exists", er)
		return
	}

	hashedPassword, _ := pw.HashPassword(password)
	users[username] = Login{
		HashedPassword: hashedPassword,
	}

	//debug purposes
	fmt.Fprintln(w, "user registered")

}

func LoginFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		er := http.StatusMethodNotAllowed
		http.Error(w, "invalid request method", er)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, ok := users[username]
	if !ok || !pw.CheckPasswordHash(password, user.HashedPassword) {
		er := http.StatusUnauthorized
		http.Error(w, "invalid username or password", er)
		return
	}

	fmt.Fprintln(w, "works")
}

func Logout(w http.ResponseWriter, r *http.Request) {

}

func Protected(w http.ResponseWriter, r *http.Request) {

}
