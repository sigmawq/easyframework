package main

import (
	ef "easyframework/easyframework"
	"log"
)

type HelloWorldRequest struct {
	Data1 string
	Data2 int
}

func GetUsers(ef *ef.RequestContext, request HelloWorldRequest) (users []User, problem ef.Problem) {
	users = append(users, User{
		Name: "John",
		Age:  60,
	})

	users = append(users, User{
		Name: "Ann",
		Age:  40,
	})

	users = append(users, User{
		Name: "Jack",
		Age:  14,
	})

	return
}

func Login(ef *ef.RequestContext) {

}

type User struct {
	Name       string
	Age        int  `id:"1"`
	IsRetarded bool `id:"2"`
}

func main() {
	server := ef.Initialize()
	server.Port = 10000

	ef.NewRPC(&server, "getUsers", GetUsers)

	theUser := User{
		Name: "Jack",
		Age:  14,
	}
	log.Println(ef.Pack(theUser))

	ef.StartServer(&server)
}
