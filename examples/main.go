package main

import (
	ef "easyframework"
	"fmt"
	"log"
	"time"
)

type Substruct struct {
	D float32 `id:"1"`
}

type UserType int8

const (
	USER_TYPE_REGULAR = 0
	USER_TYPE_ADMIN   = 1
)

type User struct {
	Age        int32     `id:"1"`
	Dead       bool      `id:"2"`
	Cringe     bool      `id:"3"`
	Substruct  Substruct `id:"4"`
	SomeString string    `id:"5"`
	Type       UserType  `id:"6"`
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

const (
	BUCKET_USERS = "Users"
)

func ListAllBuckets(ctx *ef.RequestContext) (result string, problem ef.Problem) {
	tx, _ := efContext.Database.Begin(false)
	bucket, _ := ef.GetBucket(tx, BUCKET_USERS)
	ef.Iterate(bucket, func(key ef.ID128, user *User) bool {
		fmt.Printf("[%v] %#v", user)
		return true
	})
	return
}

var efContext *ef.Context

func main() {
	context, err := ef.Initialize("db", 6969)
	if err != nil {
		log.Println(err)
		return
	}
	efContext = &context

	if true {
		err := ef.NewBucket(context, BUCKET_USERS)
		if err != nil {
			panic(err)
		}

		tx, _ := context.Database.Begin(true)
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

	ef.NewRPC(&context, ef.NewRPCParams{
		Name:    "login",
		Handler: Login,
	})

	ef.NewRPC(&context, ef.NewRPCParams{
		Name:    "logout",
		Handler: Logout,
	})

	ef.NewRPC(&context, ef.NewRPCParams{
		Name:    "listBucket",
		Handler: ListAllBuckets,
	})

	ef.StartServer(&context)
}
