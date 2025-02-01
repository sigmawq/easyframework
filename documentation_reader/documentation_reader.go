package main

import (
	"fmt"
	"net/http"
	"syscall/js"
	"io"
	"bytes"
	"encoding/json"
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
	fmt.Println("enter main callback")

	doc := js.Global().Get("document")
	value := doc.Call("getElementById", "search_input").Get("value").String()
	type Request struct {
		Filter string
	}
	request, _ := json.Marshal(&Request {
		Filter: value,
	})
	
	fmt.Println("value is", value)

	go func() {
		fmt.Println("enter secondary function")
		response, _ := http.Post("rpc/docs.md", "application/json", bytes.NewReader(request))
		fmt.Println("end call")
	
		if response != nil {
			responseBody, _ := io.ReadAll(response.Body)
			fmt.Println("get doc")
			doc := js.Global().Get("document")
			
			fmt.Println("get el")
			body := doc.Call("getElementById", "documentation_body")
			
			text := string(responseBody)
			if text == "" {
				text = "Nothing found"
			}
			
			fmt.Println("update inner html")
			body.Set("innerHTML", text)
			
			fmt.Println("exit secondary function")
		}
	}()
	
	fmt.Println("exit main callback")
	
	return nil
}