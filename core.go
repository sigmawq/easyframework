package easyframework

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Context struct {
	Procedures    map[string]Procedure
	StaticData    map[string]string
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
	Identifier                   string
	Procedure                    reflect.Value
	InputType                    reflect.Type
	OutputType                   reflect.Type
	ErrorType                    reflect.Type
	Calls                        uint64
	AuthorizationNotRequired     bool
	Description                  string
	Category                     string
	Documentation                string
	NoAutomaticResponseOnSuccess bool // @TODO: better system for static content
	UserData                     interface{}
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
	ctx.StaticData = make(map[string]string)

	CreateDirectoryIfDoesntExist("logs")

	if params.DatabasePath != "" { // Setup database
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

type ValidateDataError struct {
	Field  string
	Reason string
}

func ValidateRequestStruct(errorList *[]ValidateDataError, typeof reflect.Type, valueof reflect.Value, fieldPrefix string) {
	switch typeof.Kind() {
	case reflect.Pointer:
		ValidateRequestStruct(errorList, typeof.Elem(), valueof.Elem(), fieldPrefix)
	case reflect.Struct: // TODO: Use preprocessed struct
		for i := 0; i < typeof.NumField(); i += 1 {
			fieldType := typeof.Field(i)
			fieldValue := valueof.Field(i)

			ourTags := ParseOurTags(fieldType)

			_jsonTags := fieldType.Tag.Get("json")
			jsonTags := strings.Split(_jsonTags, ",")
			name := ""
			if len(jsonTags) > 0 {
				name = jsonTags[0]
			}
			if name == "" {
				name = fieldValue.Type().Name()
			}

			if ourTags.IsARequiredField {
				if fieldValue.IsZero() {
					*errorList = append(*errorList, ValidateDataError{
						Field:  fieldPrefix + name,
						Reason: "field is missing",
					})
				}
			}

			ValidateRequestStruct(errorList, fieldType.Type, fieldValue, fieldType.Name+"/")
		}
	case reflect.Slice:
		for i := 0; i < valueof.Len(); i += 1 {
			elemTypeof := typeof.Elem()
			elemValueof := valueof.Index(i)

			ValidateRequestStruct(errorList, elemTypeof, elemValueof, typeof.Name()+"["+strconv.Itoa(i)+"]"+"/")
		}
	}
}

func (ef *Context) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	now := time.Now()

	requestID := NewID128().String()
	data, _ := io.ReadAll(request.Body)
	log.Printf("[In] %v (%v): %v", request.RequestURI, requestID, string(data))

	rpcIndex := strings.Index(request.RequestURI, "/rpc/")
	// @TODO: Authorization for static content?
	if rpcIndex == -1 { // Serve static content
		staticName := strings.TrimLeft(request.RequestURI, "/")
		filepath, ok := ef.StaticData[staticName]
		if !ok {
			RJson(writer, 400, Problem{
				ErrorID: ERROR_STATIC_CONTENT_NOT_FOUND,
			})
			return
		}
		http.ServeFile(writer, request, filepath)
		return
	}
	procedureName := request.RequestURI[rpcIndex+len("/rpc/"):]

	procedure, procedureFound := ef.Procedures[procedureName]
	if !procedureFound {
		RJson(writer, 400, Problem{
			ErrorID: ERROR_PROCEDURE_NOT_FOUND,
		})
		log.Println("[Procedure not found]")
		return
	}

	requestContext := RequestContext{
		Procedure:      &procedure,
		ResponseWriter: writer,
		Request:        request,
		RequestID:      requestID,
	}
	if !procedure.AuthorizationNotRequired {
		if ef.Authorization != nil {
			if !ef.Authorization(&requestContext, writer, request) {
				RJson(writer, 400, Problem{
					ErrorID: ERROR_AUTHENTICATION_FAILED,
					Message: "Unauthorized",
				})
				return
			}
		}
	}

	var args []reflect.Value
	if procedure.InputType != nil { // 2 input args (context, request) scenario
		requestInput := reflect.New(procedure.InputType)

		if len(data) > 0 { // we don't want to fail on zero length body
			err := json.Unmarshal(data, requestInput.Interface())
			if err != nil {
				RJson(writer, 400, Problem{
					ErrorID: ERROR_JSON_UNMARSHAL,
					Message: err.Error(),
				})
				return
			}
		}

		// validate input
		var errorList []ValidateDataError
		ValidateRequestStruct(&errorList, requestInput.Type(), requestInput, "")
		if len(errorList) > 0 {
			validationProblem := ValidationErrorProblem{}
			validationProblem.ErrorID = ERROR_VALIDATION_FAILED
			validationProblem.ValidationProblem = errorList
			RJson(writer, 400, validationProblem)
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
		if !procedure.NoAutomaticResponseOnSuccess {
			if procedure.OutputType != nil {
				RJson(writer, 200, response.Interface())
			}
		}
	} else {
		RJson(writer, 400, problem.Interface())
	}

	then := time.Now()
	diff := then.Sub(now)
	log.Printf("[Out, %v] %v (%v)", diff, procedureName, requestID)
}

type RequestContext struct {
	Procedure      *Procedure
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	RequestID      string
	SessionToken   string
}

type NewRPCParams struct {
	Name                         string
	Handler                      interface{}
	AuthorizationNotRequired     bool
	Description                  string
	Category                     string
	NoAutomaticResponseOnSuccess bool
	UserData                     interface{}
}

func NewRPC(efContext *Context, params NewRPCParams) {
	_, identifierIsInUse := efContext.Procedures[params.Name]
	if identifierIsInUse {
		log.Printf("NewRPC(): procedure identifier already in use: %v", params.Name) // @TODO: Better error messages
		panic("Cannot continue!")
	}

	handlerTypeof := reflect.TypeOf(params.Handler)
	if handlerTypeof.Kind() != reflect.Func {
		log.Printf("NewRPC(): non-procedure passed as handler to %v", params.Name)
		panic("Cannot continue!")
	}

	if handlerTypeof.NumIn() > 2 {
		log.Println("NewRPC(): input signature is not correct, expected (*RequestContext, (any type) <- optional) as input signature", params.Name)
		panic("Cannot continue!")
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
		panic("Cannot continue!")
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
		panic("Cannot continue!")
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
					if field.Type.Name() == "Problem" {
						hasEfError = true
						break
					}
				}
			}
		}

		if !hasEfError {
			log.Printf("[%v] Problem should be embedded in output error struct (or be the error struct that handler returns)", params.Name)
			panic("NewRPC() failed")
		}
	}

	category := params.Category
	if category == "" {
		category = "Unknown category"
	}
	procedure := Procedure{
		Identifier:                   params.Name,
		Procedure:                    reflect.ValueOf(params.Handler),
		InputType:                    inputTypeOf,
		OutputType:                   outputTypeof,
		ErrorType:                    errorTypeof,
		AuthorizationNotRequired:     params.AuthorizationNotRequired,
		Description:                  params.Description,
		Category:                     params.Category,
		NoAutomaticResponseOnSuccess: params.NoAutomaticResponseOnSuccess,
		UserData:                     params.UserData,
	}
	{ // Generate procedure documentation
		var sb strings.Builder

		sb.WriteString(fmt.Sprintf("<h3 class=\"leftpad_10\"> <b>URL: rpc/%v</b> </h3>\n", procedure.Identifier))

		sb.WriteString("<div class=\"rpc_description\">\n")

		if procedure.Description != "" {
			sb.WriteString(fmt.Sprintf("<b>Description</b>: %v\n", procedure.Description))
		}

		sb.WriteString("<h4>Request:</h2>\n")
		sb.WriteString("<code>")

		if procedure.InputType != nil {
			TypeToMarkdown(&sb, reflect.New(procedure.InputType).Interface())
		} else {
			sb.WriteString("empty")
		}
		sb.WriteString(fmt.Sprintf("</code>"))

		sb.WriteString("<h4>Response:</h2>\n")
		sb.WriteString("<code>")

		if params.NoAutomaticResponseOnSuccess {
			sb.WriteString("Custom response\n")
		} else if procedure.OutputType != nil {
			TypeToMarkdown(&sb, reflect.New(procedure.OutputType).Interface())
		} else {
			sb.WriteString("empty\n")
		}
		sb.WriteString(fmt.Sprintf("</code>"))

		sb.WriteString("</div>\n")

		sb.WriteString("<hr class=\"solid\">")
		sb.WriteString("\n")

		procedure.Documentation = sb.String()
	}

	efContext.Procedures[params.Name] = procedure
}

func StaticContent(context *Context, name, filepath string) {
	// @TODO: Validate that path exists
	context.StaticData[name] = filepath
}

type ErrorID string

const (
	ERROR_NONE                     ErrorID = "none"
	ERROR_PROCEDURE_NOT_FOUND              = "procedure_not_found"
	ERROR_JSON_UNMARSHAL                   = "json_unmarshal_failed"
	ERROR_VALIDATION_FAILED                = "request_validation_failed"
	ERROR_INTERNAL                         = "internal_error"
	ERROR_AUTHENTICATION_FAILED            = "authentication_failed"
	ERROR_STATIC_CONTENT_NOT_FOUND         = "static_content_not_found"
	ERROR_REST_PROCEDURE_NOT_FOUND         = "rest_procedure_not_found"
)

type Problem struct {
	ErrorID ErrorID
	Message string
}

type ValidationErrorProblem struct {
	Problem
	ValidationProblem []ValidateDataError
}

func StartServer(efContext *Context) {
	log.Printf("%v procedures registered", len(efContext.Procedures))
	log.Printf("Listen of port %v", efContext.Port)

	http.ListenAndServe(fmt.Sprintf(":%v", efContext.Port), efContext)
}

type ID128 [16]byte

func (id ID128) String() string {
	table := [32]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', '1', '2', '3', '4', '5', '6'}
	var str [len(id) * 2]byte
	k := 0
	for i := 0; i < len(id); i++ {
		low4bits := id[i] & 0b00001111
		lowChar := byte(unicode.ToUpper(rune(table[low4bits])))

		high4bits := (id[i] & 0b11110000) >> 4
		highChar := table[high4bits]

		str[k] = highChar
		str[k+1] = lowChar
		k += 2
	}

	return string(str[:])
}

func (id *ID128) FromString(str string) error {
	table := [255]byte{ // Sub 1 when using it to get the value, zero value is used to indicate an absent characher
		97:  1,
		98:  2,
		99:  3,
		100: 4,
		101: 5,
		102: 6,
		103: 7,
		104: 8,
		105: 9,
		106: 10,
		49:  11,
		50:  12,
		51:  13,
		52:  14,
		53:  15,
		54:  16,
	}
	if len(str) != (len(*id) * 2) {
		return errors.New("ID128: Wrong size")
	}
	str = strings.ToLower(str)

	data := ([]byte)(str)
	k := 0
	for i := 0; i <= len(data)-1; i += 2 {
		highChar := data[i]
		lowChar := data[i+1]

		highValue := table[highChar]
		lowValue := table[lowChar]
		if highValue == 0 || lowValue == 0 {
			return errors.New("Invalid character in base32 encoding!")
		}

		id[k] = ((highValue - 1) << 4) | (lowValue - 1)
		k += 1
	}

	return nil
}

func (id ID128) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(id.String()))
}

func (id ID128) UnmarshalJSON(src []byte) error {
	if bytes.Equal(src, []byte("null")) {
		return nil // no error
	}
	var strValue string
	var err error
	err = json.Unmarshal(src, &strValue)
	if err != nil {
		return err
	}
	err = id.FromString(strValue)
	return err
}

func NewID128() ID128 {
	var id ID128

	rand.Read(id[:])
	return id
}
