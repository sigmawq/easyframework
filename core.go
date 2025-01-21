package easyframework

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type Context struct {
	Procedures map[string]Procedure
	Port       int
}

type Procedure struct {
	Identifier string
	Procedure  reflect.Value
	InputType  reflect.Type
	OutputType reflect.Type
	ErrorType  reflect.Type
	Calls      uint64
}

func Initialize() Context {
	return Context{
		Procedures: make(map[string]Procedure),
	}

}

func RJson[T any](w http.ResponseWriter, status int, value T) {
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("Error while trying to marshal json to send it as response: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, string(data))
}

func (ef *Context) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	now := time.Now()

	procedureName := strings.TrimLeft(request.RequestURI, "/")
	requestID := GenerateSixteenDigitCode()
	log.Printf("Incoming request: %v (%v)", procedureName, requestID)
	procedure, procedureFound := ef.Procedures[procedureName]
	if !procedureFound {
		RJson(writer, 400, Problem{
			ErrorID: ERROR_PROCEDURE_NOT_FOUND,
		})
		log.Println("[Procedure not found]")
		return
	}
	data, _ := io.ReadAll(request.Body)

	requestContext := RequestContext{
		RequestID: requestID,
	}
	requestInput := reflect.New(procedure.InputType)
	err := json.Unmarshal(data, requestInput.Interface())
	if err != nil {
		RJson(writer, 400, Problem{
			ErrorID: ERROR_JSON_UNMARSHAL,
			Message: err.Error(),
		})
		return
	}

	args := []reflect.Value{
		reflect.ValueOf(&requestContext),
		requestInput.Elem(),
	}
	returnValues := procedure.Procedure.Call(args)
	var response reflect.Value
	var problem reflect.Value
	if procedure.OutputType != nil {
		response = returnValues[0]
		problem = returnValues[1]
	} else {
		problem = returnValues[0]
	}

	errorCode := ERROR_NONE
	LookupErrorCode := func(value reflect.Value) { // Problem code should be at most one layer deep in an anonymous (nested) struct
		_type := value.Type()
		for fieldI := 0; fieldI < value.NumField(); fieldI += 1 {
			fieldValue := value.Field(fieldI)
			fieldType := _type.Field(fieldI)
			if fieldType.Anonymous && fieldType.Name == "Problem" {
				for fieldK := 0; fieldK < fieldValue.NumField(); fieldK += 1 {
					fieldValue := fieldValue.Field(fieldK)
					fieldType := fieldType.Type.Field(fieldK)

					if fieldType.Name == "ErrorID" {
						errorCode = ErrorID(fieldValue.String())
						return
					}
				}
			}

			if fieldType.Name == "ErrorID" {
				errorCode = ErrorID(fieldValue.String())
				return
			}
		}
	}
	LookupErrorCode(problem)

	if errorCode == ERROR_NONE || errorCode == "" {
		if procedure.OutputType != nil {
			RJson(writer, 200, response.Interface())
		}
	} else {
		RJson(writer, 400, problem.Interface())
	}

	then := time.Now()
	diff := then.Sub(now)
	log.Printf("(%v) %v (%v)", diff, procedureName, requestID)
}

type RequestContext struct {
	RequestID string
}

func NewRPC(efContext *Context, name string, handler interface{}) {
	_, identifierIsInUse := efContext.Procedures[name]
	if identifierIsInUse {
		log.Printf("NewRPC(): procedure identifier already in use: %v", name)
		panic("Cannot continue further!")
	}

	handlerTypeof := reflect.TypeOf(handler)
	if handlerTypeof.Kind() != reflect.Func {
		log.Printf("NewRPC(): non-procedure passed as handler to %v", name)
		panic("Cannot continue further!")
	}

	if handlerTypeof.NumIn() != 2 {
		log.Println("NewRPC(): input signature is not correct, expected (*EF_RequestContext, (any type)) as input signature", name)
		panic("Cannot continue further!")
	}

	contextTypeof := handlerTypeof.In(0)
	contextTypeofValidated := false
	if contextTypeof.Kind() == reflect.Pointer {
		if contextTypeof.Elem().Name() == "RequestContext" {
			contextTypeofValidated = true
		}
	}

	if !contextTypeofValidated {
		log.Println("NewRPC(): handler has an invalid signature, the first argument should be *EF_RequestContext")
		panic("Cannot continue further!")
	}

	inputTypeof := handlerTypeof.In(1)

	var outputTypeof reflect.Type
	var errorTypeof reflect.Type
	if handlerTypeof.NumOut() == 2 {
		outputTypeof = handlerTypeof.Out(0)
		errorTypeof = handlerTypeof.Out(1)
	} else if handlerTypeof.NumOut() == 1 {
		errorTypeof = handlerTypeof.Out(0)
	} else {
		log.Println("NewRPC(): handler has an invalid signature, 1 or 2 output arguments are allowed.")
		panic("Cannot continue further!")
	}

	if errorTypeof.Kind() != reflect.Struct {
		log.Println("NewRPC(): handler has an invalid signature, the error output should be a struct")
		hasEfError := false

		if errorTypeof.Name() == "Problem" { // Problem struct can either be returned directly or embedded one level deep inside a different struct
			hasEfError = true
		}

		if !hasEfError {
			for fieldI := 0; fieldI < errorTypeof.NumField(); fieldI += 1 {
				field := errorTypeof.Field(fieldI)
				if field.Anonymous {
					if field.Type.Name() == "EF_Problem" {
						hasEfError = true
						break
					}
				}
			}
		}

		if !hasEfError {
			log.Println("NewRPC(): Problem should be embedded in output error struct (or be the error struct that handler returns)")
			panic("Cannot continue further!")
		}
	}

	procedure := Procedure{
		Identifier: name,
		Procedure:  reflect.ValueOf(handler),
		InputType:  inputTypeof,
		OutputType: outputTypeof,
		ErrorType:  errorTypeof,
	}

	efContext.Procedures[name] = procedure
}

type ErrorID string

const (
	ERROR_NONE                ErrorID = "none"
	ERROR_PROCEDURE_NOT_FOUND         = "procedure_not_found"
	ERROR_JSON_UNMARSHAL              = "json_unmarshal_failed"
	ERROR_INTERNAL                    = "internal_error"
)

type Problem struct {
	ErrorID ErrorID
	Message string
}

func StartServer(efContext *Context) {
	http.ListenAndServe(fmt.Sprintf(":%v", efContext.Port), efContext)
}
