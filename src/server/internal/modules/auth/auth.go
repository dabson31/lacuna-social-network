package auth

// auth — register, login, logout, session issuing. Owns the credential half of a user.

// import (
// 	"errors"
// 	"fmt"
// 	"net/http"

// 	pw "social-network/internal/platform/password"
// )

// type Login struct {
// 	HashedPassword string
// 	SessionToken   string
// 	CSRFToken      string
// }

// var Users = map[string]Login{}

// func Register(w http.ResponseWriter, r *http.Request) {

// 	username := r.FormValue("username")
// 	password := r.FormValue("password")

// 	if len(username) < 8 || len(password) < 8 {
// 		er := http.StatusNotAcceptable
// 		http.Error(w, "Invalid username/password length", er)
// 		return
// 	}

// 	// replace with db
// 	if _, ok := Users[username]; ok {
// 		er := http.StatusConflict
// 		http.Error(w, "User already exists", er)
// 		return
// 	}

// 	hashedPassword, _ := pw.HashPassword(password)
// 	Users[username] = Login{
// 		HashedPassword: hashedPassword,
// 	}

// 	//debug purposes
// 	fmt.Fprintln(w, "user registered")

// }

// func LoginFunc(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		er := http.StatusMethodNotAllowed
// 		http.Error(w, "invalid request method", er)
// 		return
// 	}

// 	username := r.FormValue("username")
// 	password := r.FormValue("password")

// 	user, ok := Users[username]
// 	if !ok || !pw.CheckPasswordHash(password, user.HashedPassword) {
// 		er := http.StatusUnauthorized
// 		http.Error(w, "invalid username or password", er)
// 		return
// 	}

// 	sessionToken := pw.GenerateToken(32)

// 	http.SetCookie(w, &http.Cookie{
// 		Name:  "session_token",
// 		Value: sessionToken,
// 	})

// 	user.SessionToken = sessionToken
// 	Users[username] = user

// 	fmt.Fprintln(w, "works")
// }

// func Logout(w http.ResponseWriter, r *http.Request) {
// 	if err := Authorize(r); err != nil {
// 		er := http.StatusUnauthorized
// 		http.Error(w, "Unauthorized", er)
// 		return
// 	}
// }

// func Protected(w http.ResponseWriter, r *http.Request) {

// }

// // i think this should be in session/session.go but ill keep it here for now
// func Authorize(r *http.Request) error {
// 	username := r.FormValue("username")
// 	user, ok := Users[username]
// 	if !ok {
// 		return errors.New("Unauthorized")
// 	}

// 	st, err := r.Cookie("session_token")
// 	if err != nil || st.Value == "" || st.Value != user.SessionToken {
// 		return errors.New("Unauthorized")
// 	}

// 	csrf := r.Header.Get("X-CSRF-Token")
// 	if csrf == "" || csrf != user.CSRFToken {
// 		return errors.New("Unauthorized")
// 	}

// 	return nil
// }
