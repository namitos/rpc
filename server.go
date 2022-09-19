package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"sync"

	"github.com/namitos/rpc/packets"
	"github.com/namitos/rpc/schema"
)

type Server struct {
	sync.Map
	AllowOrigins []string
	Logging      bool

	listener net.Listener
	closed   bool
}

type methodHandler struct {
	fn           interface{}
	inputType    reflect.Type
	resultType   reflect.Type
	methodSchema *MethodSchema
}

func (h *methodHandler) unmarshalInput(inputMessage json.RawMessage) (reflect.Value, error) {
	var input reflect.Value
	var inputPtr reflect.Value
	if h.inputType.Kind() == reflect.Ptr {
		input = reflect.New(h.inputType.Elem())
		inputPtr = input
	} else {
		inputPtr = reflect.New(h.inputType)
		input = inputPtr.Elem()
	}
	if inputMessage == nil {
		return input, nil
	}
	if err := json.Unmarshal(inputMessage, inputPtr.Interface()); err != nil {
		return input, err
	}
	return input, nil
}

func (h *Server) Set(name string, fn interface{}) {
	fnType := reflect.ValueOf(fn).Type()
	if fnType.Kind() != reflect.Func {
		log.Fatalf("%v should be a Func type", name)
	}
	var inputType reflect.Type
	var params []MethodSchemaParam
	if fnType.NumIn() > 0 {
		inputType = fnType.In(0)
		inputTypeForSchema := inputType
		if inputType.Kind() == reflect.Ptr {
			inputTypeForSchema = inputType.Elem()
		}
		params = append(params, MethodSchemaParam{
			Name:     "Params",
			Schema:   schema.Get(inputTypeForSchema),
			Required: true,
		})
	}

	var resultType reflect.Type
	if fnType.NumOut() > 0 {
		resultType = fnType.Out(0)
		if resultType.Kind() == reflect.Ptr {
			resultType = resultType.Elem()
		}
	}

	h.Store(name, &methodHandler{
		fn:         fn,
		inputType:  inputType,
		resultType: resultType,
		methodSchema: &MethodSchema{
			Name:   name,
			Params: params,
			Result: MethodSchemaParam{
				Schema: schema.Get(resultType),
			},
		},
	})
}

func (h *Server) Get(name string) (*methodHandler, error) {
	method, ok := h.Load(name)
	if !ok {
		return nil, fmt.Errorf("method not found")
	}
	method1, ok := method.(*methodHandler)
	if !ok {
		return nil, fmt.Errorf("method not found")
	}
	return method1, nil
}

func (h *Server) GetMethodSchema(name string) (*MethodSchema, error) {
	mh, err := h.Get(name)
	if err != nil {
		return nil, err
	}
	return mh.methodSchema, nil
}

func (h *Server) GetAllMethods() []string {
	var methods []string
	h.Range(func(k, v interface{}) bool {
		methods = append(methods, k.(string))
		return true
	})
	return methods
}

type Input struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	JsonRPC string      `json:"jsonrpc,omitempty"`
	ID      string      `json:"id,omitempty"`
}

type inputPartial struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type Output struct {
	Result  interface{}  `json:"result,omitempty"`
	Error   *OutputError `json:"error,omitempty"`
	JsonRPC string       `json:"jsonrpc,omitempty"`
	ID      string       `json:"id,omitempty"`
}
type OutputError struct {
	Code    int64       `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *OutputError) Error() string {
	eJSON, _ := json.Marshal(e)
	return string(eJSON)
}

func (h *Server) handleTCPConnection(connection net.Conn) {
	for {
		message, messageType, messageID, length, err := packets.Parse(connection)
		if err != nil {
			if h.Logging {
				log.Println("RPCServer packets.Parse", err)
			}
			return
		}
		if h.Logging {
			log.Printf("RPCServer message: messageID %v; length %v;", messageID, length)
		}
		go h.handleTCPConnectionBytes(connection, message, messageType, messageID) //running different calls of single connection in different routines
	}
}

func (h *Server) handleTCPConnectionBytes(connection net.Conn, message []byte, messageType uint64, messageID uint64) {
	r, err := h.HandleBytes(message, messageID)
	if err != nil {
		if h.Logging {
			log.Println("RPCServer HandleBytes", err)
		}
		errJSON, _ := json.Marshal(map[string]interface{}{"error": err.Error(), "messageID": messageID})
		connection.Write(packets.Create(errJSON, messageType, messageID))
	} else {
		connection.Write(packets.Create(r, messageType, messageID))
	}
}

func (h *Server) HandleBytes(bodyBytes []byte, messageID uint64) ([]byte, error) {
	if len(bodyBytes) == 0 {
		return nil, fmt.Errorf("zero bytes handled")
	}
	var input []*inputPartial
	var arrayInput bool

	if bodyBytes[0] == 91 { //'['
		err := json.Unmarshal(bodyBytes, &input)
		if err != nil {
			return nil, err
		}
		if len(input) == 0 { //skip wg and avoid json.Marshal panic with nil input
			return []byte("[]"), nil
		}
		arrayInput = true
	} else if bodyBytes[0] == 123 { //'{'
		input1 := &inputPartial{}
		err := json.Unmarshal(bodyBytes, input1)
		if err != nil {
			return nil, err
		}
		input = append(input, input1)
	} else {
		return nil, fmt.Errorf("firstSymbol is not a json part")
	}

	wg := sync.WaitGroup{}
	wg.Add(len(input))
	results := make([]*Output, len(input))
	for i, inputItem := range input {
		go func(i int, inputItem *inputPartial) {
			defer wg.Done()
			if h.Logging {
				log.Printf("RPCServer run method: messageID %v; method %v", messageID, inputItem.Method)
			}
			output := &Output{}
			results[i] = output

			method, err := h.Get(inputItem.Method)
			if err != nil {
				output.Error = &OutputError{Message: err.Error()}
				return
			}

			var methodOut []reflect.Value
			if method.inputType == nil {
				methodOut = reflect.ValueOf(method.fn).Call(nil)
			} else {
				params, err := method.unmarshalInput(inputItem.Params)
				if err != nil {
					log.Println("err", err)
					output.Error = &OutputError{Message: err.Error()}
					return
				}
				methodOut = reflect.ValueOf(method.fn).Call([]reflect.Value{params})
			}
			if len(methodOut) > 0 {
				output.Result = methodOut[0].Interface()
			}
			if len(methodOut) > 1 {
				errInterface := methodOut[1].Interface()
				if errInterface != nil {
					err1, ok := errInterface.(*OutputError)
					if ok {
						output.Error = err1
					}
					err, ok := errInterface.(error)
					if ok {
						output.Error = &OutputError{Message: err.Error()}
					}
					return
				}
			}
		}(i, inputItem)
	}
	wg.Wait()

	if arrayInput {
		resultJSON, err := json.Marshal(results)
		if err != nil {
			return nil, err
		}
		return resultJSON, nil
	}
	resultJSON, err := json.Marshal(results[0])
	if err != nil {
		return nil, err
	}
	return resultJSON, nil
}

func setDefaultHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

func sendApiError(w http.ResponseWriter, err error) {
	setDefaultHeaders(w)
	output, _ := json.Marshal(Output{
		Error: &OutputError{Message: err.Error()},
	})
	w.Write(output)
}

func (h *Server) setCORSHeaders(w http.ResponseWriter, r *http.Request) bool {
	setDefaultHeaders(w)
	headers := w.Header()
	allowOrigins := h.AllowOrigins
	allowOrigin := ""
	origin := r.Header.Get("Origin")
	if len(allowOrigins) == 0 {
		allowOrigin = "*"
	} else {
		for _, o := range allowOrigins {
			if o == origin {
				allowOrigin = o
				break
			}
		}
	}
	if allowOrigin != "" {
		headers.Set("Access-Control-Allow-Origin", allowOrigin)
	}
	if r.Method == "OPTIONS" {
		headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		return true
	}
	return false
}

func (h *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	write := h.setCORSHeaders(w, r)
	if write {
		w.Write([]byte("{}"))
		return
	}
	if r.Method == "GET" {
		resultJSON, err := json.Marshal(h.GetAllMethods())
		if err != nil {
			sendApiError(w, err)
			return
		}
		w.Write(resultJSON)
		return
	}
	if r.Method == "POST" {
		bodyBytes, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			sendApiError(w, err)
			return
		}
		resultJSON, err := h.HandleBytes(bodyBytes, 0)
		if err != nil {
			sendApiError(w, err)
			return
		}
		w.Write(resultJSON)
		return
	}
	sendApiError(w, fmt.Errorf("not implemented"))
}

func (h *Server) ListenHTTP(port string) error {
	http.HandleFunc("/api/rpc", h.HandleHTTP)
	log.Println("RPCServer.ListenHTTP", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		return err
	}
	return nil
}

func (h *Server) ListenTCP(port string) error {
	l, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		return err
	}
	h.listener = l
	h.closed = false
	log.Println("RPCServer.ListenTCP", port)
	defer h.listener.Close()
	for {
		if h.listener == nil {
			return fmt.Errorf("listener not exists")
		}
		connection, err := h.listener.Accept()
		if err != nil {
			if h.Logging {
				log.Println("connection accept error", err)
			}
			continue
		}
		go h.handleTCPConnection(connection)
	}
}

func (h *Server) CloseTCP() error {
	err := h.listener.Close()
	h.listener = nil
	return err
}
