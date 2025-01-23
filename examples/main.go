package main

import (
	ef "easyframework"
	"fmt"
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
	Timestamp  uint64    `id:"7"`
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

func Logout(ctx *ef.RequestContext) (problem ef.Problem) {
	return
}

const (
	BUCKET_USERS = "Users"
)

func ListAllBuckets(ctx *ef.RequestContext) (result []interface{}, problem ef.Problem) {
	tx, _ := efContext.Database.Begin(false)
	bucket, _ := ef.GetBucket(tx, BUCKET_USERS)

	type Record[T any] struct {
		ID     ef.ID128
		Struct T
	}

	ef.Iterate(bucket, func(userID ef.ID128, user *User) bool {
		result = append(result, Record[User]{
			userID, *user,
		})
		return true
	})

	return
}

var efContext *ef.Context

func main() {
	efContext = &ef.Context{
		Port:          6969,
		DatabasePath:  "db",
		Authorization: nil,
		StdoutLogging: true,
		FileLogging:   false,
	}

	err := ef.Initialize(efContext)
	if err != nil {
		panic(err)
	}

	if false {
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
		Name:    "login",
		Handler: Login,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:    "logout",
		Handler: Logout,
	})

	ef.NewRPC(efContext, ef.NewRPCParams{
		Name:    "listBuckets",
		Handler: ListAllBuckets,
	})

	ef.StartServer(efContext)
}
