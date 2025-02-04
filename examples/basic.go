package main

import (
	"fmt"
	ef "github.com/sigmawq/easyframework"
	"log"
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
	Age               int32        `id:"1"`
	Dead              bool         `id:"2"`
	Cringe            bool         `id:"3"`
	Substruct         Substruct    `id:"4"`
	SomeString        string       `id:"5"`
	Type              UserType     `id:"6"`
	Timestamp         uint64       `id:"7"`
	PreferredNumbers  [2]int64     `id:"8"`
	ArrayOfSubstructs [5]Substruct `id:"9"`
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

func Login(ctx *ef.RequestContext, request LoginRequest) (response LoginResponse, problem ef.Problem) {
	if request.Username == "Pupa" && request.Password == "secret" {
		response.SessionToken = ef.GenerateSixteenDigitCode()
		response.Expiry = time.Now().Add(time.Hour * 24)
		return
	}

	problem.ErrorID = ERROR_INVALID_CREDENTIALS
	problem.Message = "Bad login/password"
	return
}

func Logout(ctx *ef.RequestContext) (problem ef.Problem) {
	return
}

const (
	BUCKET_USERS = "Users"
)

func ListAllBuckets(ctx *ef.RequestContext) (result []interface{}, problem ef.Problem) {
	tx, _ := efContext.Database.Begin(false)
	bucket, _ := ef.GetBucket(tx, BUCKET_USERS)

	users := ef.IterateCollectAll[User](bucket)
	result = append(result, users)

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

func main() {
	efContext = new(ef.Context)
	params := ef.InitializeParams{
		Port:          6600,
		StdoutLogging: true,
		FileLogging:   true,
		DatabasePath:  "db",
		Authorization: nil,
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

		tx, _ := efContext.Database.Begin(true)
		defer tx.Rollback()

		bucket, err := ef.GetBucket(tx, BUCKET_USERS)
		if err != nil {
			panic(err)
		}
		{
			user := User{
				Age:        61,
				Dead:       true,
				Cringe:     true,
				SomeString: "aaabbbcccddd",
				Substruct:  Substruct{D: 444.666},
				Type:       USER_TYPE_ADMIN,
				PreferredNumbers: [2]int64{
					1337, 1488,
				},
				ArrayOfSubstructs: [5]Substruct{
					{100, 200},
				},
			}

			user2 := User{
				Age:        40,
				Dead:       true,
				Cringe:     true,
				SomeString: "addfdsfdlsfkdfgdfdfgdffggdfgfhfgjhgyujtyhrg",
				Substruct:  Substruct{D: 222.666},
			}

			err := ef.Insert(bucket, ef.NewID128(), user)
			if err != nil {
				panic(err)
			}

			err = ef.Insert(bucket, ef.NewID128(), user2)
			if err != nil {
				panic(err)
			}
		}

		ef.Iterate(bucket, func(userID ef.ID128, user *User) bool {
			fmt.Printf("Unpacked user: %#v (ID %v)", user, userID)
			return true
		})

		tx.Commit()
	}

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:    "Login",
		Handler: Login,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:    "Logout",
		Handler: Logout,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:        "ListBuckets",
		Description: "Bla bla bla",
		Handler:     ListAllBuckets,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Category:    "Logs",
		Name:        "LogList",
		Description: "Get list of all logs",
		Handler:     RPC_GetLogList,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:                         "docs.md",
		Handler:                      RPC_GetDocumentation,
		AuthorizationRequired:        true,
		NoAutomaticResponseOnSuccess: true,
	})

	ef.StaticContent(efContext, "documentation_reader.html", "documentation_reader/documentation_reader.html")
	ef.StaticContent(efContext, "wasm_exec.js", "documentation_reader/wasm_exec.js")
	ef.StaticContent(efContext, "documentation_reader.wasm", "documentation_reader/documentation_reader.wasm")

	ef.StartServer(efContext)
}
