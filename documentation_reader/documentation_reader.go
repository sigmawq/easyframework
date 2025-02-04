package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"syscall/js"
)

func main() {
	fmt.Println("Get")

	response, _ := http.Get("rpc/docs.md")
	responseBody, _ := io.ReadAll(response.Body)

	doc := js.Global().Get("document")
	body := doc.Call("getElementById", "documentation_body")
	body.Set("innerHTML", string(responseBody))

	{
		var callback js.Func
		callback = js.FuncOf(OnSearchInputChange)
		js.Global().Get("document").Call("getElementById", "search_input").Call("addEventListener", "input", callback)
	}

	select {}
}

func OnSearchInputChange(this js.Value, args []js.Value) interface{} {
	doc := js.Global().Get("document")
	value := doc.Call("getElementById", "search_input").Get("value").String()
	type Request struct {
		Filter string
	}
	request, _ := json.Marshal(&Request{
		Filter: value,
	})

	fmt.Println("value is", value)

	go func() {
		response, _ := http.Post("rpc/docs.md", "application/json", bytes.NewReader(request))

		if response != nil {
			responseBody, _ := io.ReadAll(response.Body)
			doc := js.Global().Get("document")
			body := doc.Call("getElementById", "documentation_body")

			text := string(responseBody)
			if text == "" {
				text = "Nothing found"
			}
			body.Set("innerHTML", text)
		}
	}()

	return nil
}
