package main

import ef "easyframework/easyframework"

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
	Age        int
	IsRetarded bool
}

func main() {
	server := ef.Initialize()
	server.Port = 10000

	ef.NewRPC(&server, "getUsers", GetUsers)

	ef.StartServer(&server)
}
