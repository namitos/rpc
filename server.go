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
	Fn         interface{}
	InputType  reflect.Type
	ResultType reflect.Type
}

func (h *methodHandler) UnmarshalInput(inputMessage json.RawMessage) (reflect.Value, error) {
	input := reflect.New(h.InputType)
	if inputMessage == nil {
		return input, nil
	}
	err := json.Unmarshal(inputMessage, input.Interface())
	if err != nil {
		return reflect.Value{}, err
	}
	return input, nil
}

func (h *Server) Set(name string, fn interface{}) {
	fnType := reflect.ValueOf(fn).Type()
	if fnType.Kind() != reflect.Func {
		log.Fatalf("%v should be a Func type", name)
	}
	in := fnType.In(0)
	if in.Kind() != reflect.Ptr {
		log.Fatalf("%v first argument should be a Ptr type", name)
	}
	resultType := fnType.Out(0)
	if resultType.Kind() == reflect.Ptr {
		resultType = resultType.Elem()
	}
	h.Store(name, &methodHandler{
		Fn:         fn,
		InputType:  in.Elem(),
		ResultType: resultType,
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
	inputTypeItem := reflect.New(mh.InputType)
	return &MethodSchema{
		Name: name,
		Params: []MethodSchemaParam{{
			Name:     "Params",
			Schema:   schema.Get(inputTypeItem),
			Required: true,
		}},
		Result: MethodSchemaParam{
			Schema: schema.Get(reflect.New(mh.ResultType)),
		},
	}, nil
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
	Jsonrpc string       `json:"jsonrpc,omitempty"`
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
	results := make([]interface{}, len(input))
	for i, inputItem := range input {
		go func(i int, inputItem *inputPartial) {
			defer wg.Done()
			if h.Logging {
				log.Printf("RPCServer run method: messageID %v; method %v", messageID, inputItem.Method)
			}
			method, err := h.Get(inputItem.Method)
			if err != nil {
				results[i] = &Output{Error: &OutputError{Message: err.Error()}}
				return
			}
			params, err := method.UnmarshalInput(inputItem.Params)
			if err != nil {
				results[i] = &Output{Error: &OutputError{Message: err.Error()}}
				return
			}
			out := reflect.ValueOf(method.Fn).Call([]reflect.Value{params})
			result := out[0].Interface()
			errInterface := out[1].Interface()
			if errInterface != nil {
				err, ok := errInterface.(error)
				if ok {
					results[i] = &Output{Error: &OutputError{Message: err.Error()}}
				}
				return
			}
			results[i] = &Output{Result: result}
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

type restError struct {
	Message string `json:"message"`
}

func setDefaultHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

func sendApiError(w http.ResponseWriter, err error) {
	setDefaultHeaders(w)
	errTxt := err.Error()
	output, _ := json.Marshal(restError{
		Message: errTxt,
	})
	status := http.StatusInternalServerError
	if errTxt == "not implemented" {
		status = http.StatusNotImplemented
	}
	if errTxt == "forbidden" {
		status = http.StatusForbidden
	}
	w.WriteHeader(status)
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
