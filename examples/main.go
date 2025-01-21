package main

import (
	ef "easyframework"
	"log"
)

type Substruct struct {
	D float32
}

type User struct {
	Age       int32     `id:"1"`
	Dead      bool      `id:"2"`
	Cringe    bool      `id:"3"`
	Substruct Substruct `id:"4"`
}

func main() {
	user := User{
		Age:  44,
		Dead: true,
	}
	objectBytes := ef.Pack(&user)
	log.Println(objectBytes)
}
