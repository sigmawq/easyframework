package easyframework

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type Context struct {
	Procedures    map[string]Procedure
	Port          int
	Database      *bolt.DB
	Authorization func(RequestContext, http.ResponseWriter, *http.Request) bool
}

type Procedure struct {
	Identifier            string
	Procedure             reflect.Value
	InputType             reflect.Type
	OutputType            reflect.Type
	ErrorType             reflect.Type
	Calls                 uint64
	AuthorizationRequired bool
}

func Initialize(databasePath string, port int) (Context, error) {
	database, err := bolt.Open(databasePath, 0777, nil)
	if err != nil {
		return Context{}, err
	}

	return Context{
		Procedures: make(map[string]Procedure),
		Port:       port,
		Database:   database,
	}, nil
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
	requestID := NewID128().String()
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
	if procedure.AuthorizationRequired {
		if !Authenticate(&requestContext, writer, request) {
			RJson(writer, 400, Problem{
				ErrorID: ERROR_AUTHENTICATION_FAILED,
				Message: "Bad session token",
			})
			return
		}
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

	var args []reflect.Value
	if procedure.InputType != nil { // 2 input args (context, request) scenario
		args = []reflect.Value{
			reflect.ValueOf(&requestContext),
			requestInput.Elem(),
		}
	} else { // 1 input arg (context only) scenario
		args = []reflect.Value{
			reflect.ValueOf(&requestContext),
		}
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
	LookupErrorCode := func(value reflect.Value) { // Problem struct should be at most one layer deep in an anonymous (nested) struct. Or be the struct itself
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
	RequestID    string
	SessionToken string
}

type NewRPCParams struct {
	Name                  string
	Handler               interface{}
	AuthorizationRequired bool
}

func NewRPC(efContext *Context, params NewRPCParams) {
	_, identifierIsInUse := efContext.Procedures[params.Name]
	if identifierIsInUse {
		log.Printf("NewRPC(): procedure identifier already in use: %v", params.Name)
		panic("Cannot continue further!")
	}

	handlerTypeof := reflect.TypeOf(params.Handler)
	if handlerTypeof.Kind() != reflect.Func {
		log.Printf("NewRPC(): non-procedure passed as handler to %v", params.Name)
		panic("Cannot continue further!")
	}

	if handlerTypeof.NumIn() > 2 {
		log.Println("NewRPC(): input signature is not correct, expected (*RequestContext, (any type) <- optional) as input signature", params.Name)
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
		log.Println("NewRPC(): handler has an invalid signature, the first argument should be *RequestContext")
		panic("Cannot continue further!")
	}

	var inputTypeOf reflect.Type
	if handlerTypeof.NumIn() == 2 {
		inputTypeOf = handlerTypeof.In(1)
	}

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
		Identifier:            params.Name,
		Procedure:             reflect.ValueOf(params.Handler),
		InputType:             inputTypeOf,
		OutputType:            outputTypeof,
		ErrorType:             errorTypeof,
		AuthorizationRequired: params.AuthorizationRequired,
	}

	efContext.Procedures[params.Name] = procedure
}

type ErrorID string

const (
	ERROR_NONE                  ErrorID = "none"
	ERROR_PROCEDURE_NOT_FOUND           = "procedure_not_found"
	ERROR_JSON_UNMARSHAL                = "json_unmarshal_failed"
	ERROR_INTERNAL                      = "internal_error"
	ERROR_AUTHENTICATION_FAILED         = "authentication_failed"
)

type Problem struct {
	ErrorID ErrorID
	Message string
}

func StartServer(efContext *Context) {
	log.Printf("%v procedures registered", len(efContext.Procedures))
	log.Printf("Listen of port %v", efContext.Port)
	http.ListenAndServe(fmt.Sprintf(":%v", efContext.Port), efContext)
}

type ID128 [16]byte
type ID256 [32]byte

func (id ID128) String() string {
	return base64.URLEncoding.EncodeToString(id[:])
}

func (id ID256) String() string {
	return base64.URLEncoding.EncodeToString(id[:])
}

func NewID128() ID128 {
	var id ID128

	rand.Read(id[:])
	return id
}

func NewID256() ID256 {
	var id ID256

	rand.Read(id[:])
	return id
}
