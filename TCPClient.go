package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/namitos/rpc/packets"
)



func NewTCPClient(URL string) Client {
	client := &TCPClient{
		URL: URL,
	}
	return client
}

type TCPClient struct {
	URL string
}

func (h *TCPClient) Call(context context.Context, input *[]Input, result *[]Output) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	response, _, _, _, err := packets.Send(body, 0, 0, h.URL)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(response, result); err != nil {
		return err
	}
	return nil
}

func (h *TCPClient) CallSingle(ctx context.Context, method string, params interface{}, result interface{}) error {
	return CallSingle(h, ctx, method, params, result)
}

func NewTCPClientKeepAlive(URL string) Client {
	client := &TCPClientKeepAlive{
		URL:                URL,
		WaitingResponses:   map[uint64]chan []byte{},
		WaitingResponsesMu: sync.Mutex{},
	}
	go client.KeepAlive()
	return client
}

type TCPClientKeepAlive struct {
	URL                string
	Connection         net.Conn
	WaitingResponses   map[uint64]chan []byte
	WaitingResponsesMu sync.Mutex
	Counter            uint64
}

//KeepAlive recursive reconnects if disconnected
func (h *TCPClientKeepAlive) KeepAlive() {
	log.Println("connecting to", h.URL)
	err := h.Connect()
	h.Connection = nil
	if err != nil {
		log.Println("tcp connection disconnected", err)
		time.AfterFunc(10*time.Millisecond, func() {
			h.KeepAlive()
		})
	}
}

func (h *TCPClientKeepAlive) Connect() error {
	connection, err := net.Dial("tcp", h.URL)
	if err != nil {
		return err
	}
	h.Connection = connection
	for {
		response, _, msgID, _, err := packets.Parse(h.Connection)
		if err != nil {
			return err
		}
		h.WaitingResponsesMu.Lock()
		channel := h.WaitingResponses[msgID]
		delete(h.WaitingResponses, msgID)
		h.WaitingResponsesMu.Unlock()
		if channel != nil {
			channel <- response
			close(channel)
		}
	}
}

func (h *TCPClientKeepAlive) Call(ctx context.Context, input *[]Input, result *[]Output) error {
	if h.Connection == nil {
		return fmt.Errorf("client not connected")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	channel := make(chan []byte)
	h.WaitingResponsesMu.Lock()
	h.Counter++
	msgID := h.Counter
	h.WaitingResponses[msgID] = channel
	h.WaitingResponsesMu.Unlock()
	h.Connection.Write(packets.Create(body, 0, msgID))
	var response []byte
	select {
	case response = <-channel:
		{
			if err = json.Unmarshal(response, result); err != nil {
				return err
			}
			return nil
		}
	case <-ctx.Done():
		{
			h.WaitingResponsesMu.Lock()
			delete(h.WaitingResponses, msgID)
			close(channel)
			h.WaitingResponsesMu.Unlock()
			return ctx.Err()
		}
	}
}

func (h *TCPClientKeepAlive) CallSingle(ctx context.Context, method string, params interface{}, result interface{}) error {
	return CallSingle(h, ctx, method, params, result)
}
