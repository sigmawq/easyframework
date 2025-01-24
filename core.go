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
	"os"
	"reflect"
	"strings"
	"time"
)

type Context struct {
	Procedures    map[string]Procedure
	Port          int
	DatabasePath  string
	Database      *bolt.DB
	Authorization func(*RequestContext, http.ResponseWriter, *http.Request) bool
	StdoutLogging bool
	FileLogging   bool
	LogFile       *os.File
}

func (ctx Context) Write(bytes []byte) (int, error) {
	str := fmt.Sprintf("[%v, %v, g%v] %v", time.Now().Format("2006-01-02T15:04:05.999Z"), GetTrace(4), curGoroutineID(), string(bytes))
	if ctx.StdoutLogging {
		fmt.Print(str)
	}
	if ctx.FileLogging {
		ctx.LogFile.Write([]byte(str))
	}
	return len(bytes), nil
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

type InitializeParams struct {
	Port          int
	StdoutLogging bool
	FileLogging   bool
	DatabasePath  string
	Authorization func(*RequestContext, http.ResponseWriter, *http.Request) bool
}

func Initialize(ctx *Context, params InitializeParams) error {
	ctx.Procedures = make(map[string]Procedure)

	ctx.FileLogging = params.FileLogging
	ctx.StdoutLogging = params.StdoutLogging
	ctx.Authorization = params.Authorization
	ctx.Port = params.Port

	{ // Setup database
		database, err := bolt.Open(params.DatabasePath, 0777, nil)
		if err != nil {
			return err
		}

		ctx.Database = database
	}

	// Setup logging
	{
		log.SetFlags(0)

		var file *os.File
		var err error
		if ctx.FileLogging {
			filename := fmt.Sprintf("logs/log_%v", time.Now().Format("02_01_2006_15-04"))
			file, err = os.Create(filename)
			if err != nil {
				panic("err")
			}

			ctx.LogFile = file
		}

		log.SetOutput(ctx)
	}

	return nil
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
		if ef.Authorization != nil {
			if !ef.Authorization(&requestContext, writer, request) {
				RJson(writer, 400, Problem{
					ErrorID: ERROR_AUTHENTICATION_FAILED,
					Message: "Bad session token",
				})
				return
			}
		}

	}

	var args []reflect.Value
	if procedure.InputType != nil { // 2 input args (context, request) scenario
		requestInput := reflect.New(procedure.InputType)
		err := json.Unmarshal(data, requestInput.Interface())
		if err != nil {
			RJson(writer, 400, Problem{
				ErrorID: ERROR_JSON_UNMARSHAL,
				Message: err.Error(),
			})
			return
		}

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

func (id ID128) String() string {
	return base64.StdEncoding.EncodeToString(id[:])
}

func (id ID128) MarshalJSON() ([]byte, error) {
	var result [24]byte
	base64.StdEncoding.Encode(result[:], id[:])
	return json.Marshal(result[:])
}

func (id ID128) UnmarshalJSON(src []byte) error {
	_, err := base64.StdEncoding.Decode(id[:], src)
	return err
}

func (id ID256) String() string {
	return base64.StdEncoding.EncodeToString(id[:])
}

func (id ID256) MarshalJSON() ([]byte, error) {
	var result [48]byte
	base64.StdEncoding.Encode(result[:], id[:])
	return json.Marshal(result[:])
}

func (id ID256) UnmarshalJSON(src []byte) error {
	_, err := base64.StdEncoding.Decode(id[:], src)
	return err
}

type ID256 [32]byte

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
