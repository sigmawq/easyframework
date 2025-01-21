package main

import (
	ef "easyframework"
	"log"
	"time"
)

type Substruct struct {
	D float32 `id:"1"`
}

type User struct {
	Age        int32     `id:"1"`
	Dead       bool      `id:"2"`
	Cringe     bool      `id:"3"`
	Substruct  Substruct `id:"4"`
	SomeString string    `id:"5"`
}

type LoginRequest struct {
	Username string
	Password string
}

type LoginResponse struct {
	SessionToken string
	Expiry       time.Time
}

const (
	ERROR_INVALID_CREDENTIALS = "invalid_credentials"
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

// TODO: Empty request signature
type LogoutRequest struct{}

func Logout(ctx *ef.RequestContext, request LogoutRequest) (problem ef.Problem) {
	return
}

func main() {
	user := User{
		Age:        61,
		Dead:       true,
		Cringe:     true,
		SomeString: "aaabbbcccddd",
		Substruct:  Substruct{D: 666.666},
	}
	objectBytes, err := ef.Pack(&user)
	if err != nil {
		panic(err)
	}
	log.Println(objectBytes, len(objectBytes), "bytes")

	unpackedUser := User{}
	err = ef.Unpack(objectBytes, &unpackedUser)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("struct before serialization: ", user)
	log.Println("struct after serialization: ", unpackedUser)

	context := ef.Initialize(6969)
	ef.NewRPC(&context, ef.NewRPCParams{
		Name:    "login",
		Handler: Login,
	})

	ef.NewRPC(&context, ef.NewRPCParams{
		Name:    "logout",
		Handler: Logout,
	})

	ef.StartServer(&context)
}
