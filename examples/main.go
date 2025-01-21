package main

import (
	ef "easyframework"
	"log"
)

type Substruct struct {
	D float32
}

type User struct {
	Age        int32     `id:"1"`
	Dead       bool      `id:"2"`
	Cringe     bool      `id:"3"`
	Substruct  Substruct `id:"4"`
	SomeString string    `id:"5"`
}

func main() {
	user := User{
		Age:        61,
		Dead:       true,
		Cringe:     true,
		SomeString: "aaabbbcccddd",
	}
	objectBytes := ef.Pack(&user)
	log.Println(objectBytes, len(objectBytes), "bytes")

	unpackedUser := User{}
	err := ef.Unpack(objectBytes, &unpackedUser)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("struct before serialization: ", user)
	log.Println("struct after serialization: ", unpackedUser)

}
