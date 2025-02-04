package main

import (
	"fmt"
	ef "github.com/sigmawq/easyframework"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Substruct struct {
	D float32 `id:"1"`
	E float32 `id:"2"`
}

type UserType int8

const (
	USER_TYPE_REGULAR = 0
	USER_TYPE_ADMIN   = 1
)

type User struct {
	ID       ef.ID128 `id:"1"`
	Name     string   `id:"2"`
	Password string   `id:"3"`
	PreviousNames []string `id:"4"`
}

type LoginRequest struct {
	Username string `description:"login or email" tag:"required"`
	Password string `description:"at most several attempts per minute!" tag:"required"`
}

type LoginResponse struct {
	SessionToken string
	Expiry       time.Time
}

const (
	ERROR_BAD_URL_FORMAT      = "bad_url_format"
	ERROR_INVALID_CREDENTIALS = "invalid_credentials"
	ERROR_CONTENT_NOT_FOUND   = "content_not_found"
)

func Login(ctx *ef.RequestContext, request LoginRequest) (response Session, problem ef.Problem) {
	tx, _ := ef.WriteTx(efContext)
	defer tx.Rollback()

	users, _ := ef.GetBucket(tx, BUCKET_USERS)
	var user User
	if !ef.IterateFind(users, &user, func(id ef.ID128, value *User) bool {
		return value.Name == request.Username
	}) {
		problem.ErrorID = ERROR_INVALID_CREDENTIALS
		return
	}

	if request.Password != user.Password {
		problem.ErrorID = ERROR_INVALID_CREDENTIALS
		return
	}

	ip, _, _ := net.SplitHostPort(ctx.Request.RemoteAddr)

	sessionID := ef.NewID128()
	response = Session{
		ID:        sessionID,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
		IP:        ip,
	}

	sessions, _ := ef.GetBucket(tx, BUCKET_SESSIONS)
	ef.Insert(sessions, sessionID, &response)

	tx.Commit()

	http.SetCookie(ctx.ResponseWriter, &http.Cookie{
		Name:    "session",
		Value:   sessionID.String(),
		Expires: time.Unix(response.ExpiresAt, 0),
	})

	return
}

func Logout(ctx *ef.RequestContext) (problem ef.Problem) {
	return
}

const (
	BUCKET_USERS    = "Users"
	BUCKET_SESSIONS = "Sessions"
)

func ListAllBuckets(ctx *ef.RequestContext) (result []interface{}, problem ef.Problem) {
	tx, _ := efContext.Database.Begin(false)

	{
		bucket, _ := ef.GetBucket(tx, BUCKET_USERS)

		things := ef.IterateCollectAll[User](bucket)
		result = append(result, things)
	}

	{
		bucket, _ := ef.GetBucket(tx, BUCKET_SESSIONS)

		things := ef.IterateCollectAll[Session](bucket)
		result = append(result, things)
	}

	return
}

var efContext *ef.Context

type GetDocumentationRequest struct {
	Filter string
}

func RPC_GetDocumentation(context *ef.RequestContext, request GetDocumentationRequest) (problem ef.Problem) {
	ef.String200(context.ResponseWriter, ef.GetDocumentation(efContext, request.Filter))
	return
}

func RPC_GetStaticContent(context *ef.RequestContext) (problem ef.Problem) {
	parts := strings.Split(context.Request.RequestURI, "static/")
	if len(parts) != 2 {
		problem.ErrorID = ERROR_BAD_URL_FORMAT
		return
	}

	filename := parts[1]
	filepath := ""
	switch filename {
	case "documentation_reader.wasm":
		filepath = "documentation_reader/documentation_reader.wasm"
	case "index.html":
		filepath = "documentation_reader/index.html"
	case "wasm_exec.js":
		filepath = "documentation_reader/wasm_exec.js"
	}

	if filepath == "" {
		problem.ErrorID = ERROR_CONTENT_NOT_FOUND
		return
	}

	http.ServeFile(context.ResponseWriter, context.Request, filepath)
	return
}

func RPC_GetLogList(context *ef.RequestContext) (logList []string, problem ef.Problem) {
	logList = ef.GetLogList()
	return
}

type GetLogRequest struct {
	LogName       string `tag:"required"`
	Filter        string
	FilterBreadth int
}

func RPC_GetLog(context *ef.RequestContext, request GetLogRequest) (logtext string, problem ef.Problem) {
	logtext = ef.GetLog(request.LogName, request.Filter, request.FilterBreadth)
	return
}

type Session struct {
	ID            ef.ID128 `id:"1"`
	UserID        ef.ID128 `id:"2"`
	ExpiresAt     int64    `id:"3"`
	AccessCount   int64    `id:"4"`
	IP            string   `id:"5"`
}

func Authorization(ctx *ef.RequestContext, w http.ResponseWriter, r *http.Request) bool {
	_session, err := r.Cookie("session")
	if err != nil {
		return false
	}

	var sessionID ef.ID128
	err = sessionID.FromString(_session.Value)
	if err != nil {
		return false
	}

	var session Session
	if !ef.GetByID(efContext, BUCKET_SESSIONS, sessionID, &session) {
		return false
	}

	ip, _, _ := net.SplitHostPort(ctx.Request.RemoteAddr)
	if ip != session.IP {
		return false
	}

	var user User
	if !ef.GetByID(efContext, BUCKET_USERS, session.UserID, &user) {
		return false
	}
	session.AccessCount += 1

	ef.InsertByID(efContext, BUCKET_SESSIONS, sessionID, &session)

	log.Printf("User: %#v", user)
	log.Printf("Session: %#v", session)

	return true
}

type CustomProcedurePermission struct {
	IsAdminOnly bool
}

func main() {
	efContext = new(ef.Context)
	params := ef.InitializeParams{
		Port:          6600,
		StdoutLogging: true,
		FileLogging:   true,
		DatabasePath:  "db",
		Authorization: Authorization,
	}
	err := ef.Initialize(efContext, params)
	if err != nil {
		log.Println("Error while initializing EF:", err)
		return
	}

	{
		first := ef.NewID128()
		second := first.String()

		var third ef.ID128
		third.FromString(second)
		log.Println(first, second, third)
	}

	if true {
		err := ef.NewBucket(efContext, BUCKET_USERS)
		if err != nil {
			panic(err)
		}

		err = ef.NewBucket(efContext, BUCKET_SESSIONS)
		if err != nil {
			panic(err)
		}

		tx, _ := efContext.Database.Begin(true)
		defer tx.Rollback()

		bucket, err := ef.GetBucket(tx, BUCKET_USERS)
		if err != nil {
			panic(err)
		}
		{
			user1 := User{
				ID:       ef.NewID128(),
				Name:     fmt.Sprintf("User-%v", ef.GenerateSixteenDigitCode()),
				Password: ef.GenerateSixteenDigitCode(),
			}

			user2 := User{
				ID:       ef.NewID128(),
				Name:     fmt.Sprintf("User-%v", ef.GenerateSixteenDigitCode()),
				Password: ef.GenerateSixteenDigitCode(),
				PreviousNames: []string{
					"Previousname1",
					"Previousname2",
					"Previousname3",
				},
			}

			err := ef.Insert(bucket, user1.ID, &user1)
			if err != nil {
				panic(err)
			}

			err = ef.Insert(bucket, user2.ID, &user2)
			if err != nil {
				panic(err)
			}
		}

		tx.Commit()
	}

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:                     "Login",
		Handler:                  Login,
		AuthorizationNotRequired: true,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:    "Logout",
		Handler: Logout,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:                     "ListBuckets",
		Description:              "Bla bla bla",
		Handler:                  ListAllBuckets,
		AuthorizationNotRequired: true,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Category:    "Logs",
		Name:        "LogList",
		Description: "Get list of all logs",
		Handler:     RPC_GetLogList,
		UserData: CustomProcedurePermission{
			IsAdminOnly: true,
		},
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:                         "docs.md",
		Handler:                      RPC_GetDocumentation,
		AuthorizationNotRequired:     true,
		NoAutomaticResponseOnSuccess: true,
	})

	ef.StaticContent(efContext, "documentation_reader.html", "documentation_reader/documentation_reader.html")
	ef.StaticContent(efContext, "wasm_exec.js", "documentation_reader/wasm_exec.js")
	ef.StaticContent(efContext, "documentation_reader.wasm", "documentation_reader/documentation_reader.wasm")

	ef.StartServer(efContext)
}
