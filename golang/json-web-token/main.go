package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type M map[string]interface{}

type MyClaims struct {
	jwt.RegisteredClaims
	Username string `json:"Username"`
	Email    string `json:"Email"`
	Group    string `json:"Group"`
}

var APPLICATION_NAME = "My Simple JWT App"
var LOGIN_EXPIRATION_DURATION = time.Duration(1) * time.Hour
var JWT_SIGNING_METHOD = jwt.SigningMethodHS256
var JWT_SIGNATURE_KEY = []byte("allah is mercyfull")

func HandlerIndex(w http.ResponseWriter, r *http.Request) {
	userInfo := r.Context().Value("userInfo").(jwt.MapClaims)
	message := fmt.Sprintf("hello %s (%s)", userInfo["Username"], userInfo["Group"])
	w.Write([]byte(message))
}

func HandlerLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Unsupported http method", http.StatusBadRequest)
		return
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		http.Error(w, "Invalid username or password", http.StatusBadRequest)
		return
	}

	ok, userInfo := authenticateUser(username, password)
	if !ok {
		http.Error(w, "Invalid username or password", http.StatusBadRequest)
		return
	}

	claims := MyClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    APPLICATION_NAME,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(LOGIN_EXPIRATION_DURATION)),
		},
		Username: userInfo["username"].(string),
		Email:    userInfo["email"].(string),
		Group:    userInfo["group"].(string),
	}

	token := jwt.NewWithClaims(
		JWT_SIGNING_METHOD,
		claims,
	)

	signedToken, err := token.SignedString(JWT_SIGNATURE_KEY)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tokenString, _ := json.Marshal(M{"token": signedToken})
	w.Write([]byte(tokenString))
}

func authenticateUser(username, password string) (bool, M) {
	basePath, _ := os.Getwd()
	dbPath := filepath.Join(basePath, "users.json")
	buf, _ := os.ReadFile(dbPath)

	data := make([]M, 0)
	err := json.Unmarshal(buf, &data)
	if err != nil {
		return false, nil
	}

	for _, each := range data {
		if each["username"] == username && each["password"] == password {
			resM := make(M)
			for k, v := range each {
				resM[k] = v
			}
			delete(resM, "password")
			return true, resM
		}
	}
	return false, nil
}

func MiddlewareJWTAuthorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}

		authorizationHeader := r.Header.Get("Authorization")
		if !strings.Contains(authorizationHeader, "Bearer") {
			http.Error(w, "Invalid token", http.StatusBadRequest)
			return
		}

		tokenString := strings.Replace(authorizationHeader, "Bearer ", "", -1)

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if method, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Signing method invalid")
			} else if method != JWT_SIGNING_METHOD {
				return nil, fmt.Errorf("Signing method invalid")
			}

			return JWT_SIGNATURE_KEY, nil
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(context.Background(), "userInfo", claims)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := new(CustomMux)
	mux.RegisterMiddleware(MiddlewareJWTAuthorization)

	mux.HandleFunc("/login", HandlerLogin)
	mux.HandleFunc("/index", HandlerIndex)

	server := new(http.Server)
	server.Handler = mux
	server.Addr = ":8080"

	fmt.Println("Starting server at", server.Addr)
	server.ListenAndServe()
}
