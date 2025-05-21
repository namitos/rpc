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
	go client.KeepAlive()
	return client
}

type TCPClient struct {
	URL               string
	ReconnectInterval time.Duration

	waitingResponses   map[uint64]chan []byte
	waitingResponsesMu sync.Mutex
	connection         net.Conn
	counter            uint64
}

// KeepAlive recursive reconnects if disconnected
func (h *TCPClient) KeepAlive() {
	log.Println("connecting to", h.URL)
	err := h.Connect()
	h.connection = nil
	if err != nil {
		log.Println("tcp connection disconnected", err)
		h.waitingResponsesMu.Lock()
		for msgID, channel := range h.waitingResponses {
			delete(h.waitingResponses, msgID)
			if channel != nil {
				channel <- []byte{} //TODO: send real error; now cannot parse empty json
				close(channel)
			}
		}
		h.waitingResponsesMu.Unlock()
		if h.ReconnectInterval == 0 {
			h.ReconnectInterval = time.Second
		}
		time.AfterFunc(h.ReconnectInterval, func() {
			h.KeepAlive()
		})
	}
}

func (h *TCPClient) Connect() error {
	connection, err := net.Dial("tcp", h.URL)
	if err != nil {
		return err
	}
	h.connection = connection
	if h.waitingResponses == nil {
		h.waitingResponses = map[uint64]chan []byte{}
	}
	for {
		response, _, msgID, _, err := packets.Parse(h.connection)
		if err != nil {
			return err
		}
		h.waitingResponsesMu.Lock()
		channel := h.waitingResponses[msgID]
		delete(h.waitingResponses, msgID)
		h.waitingResponsesMu.Unlock()
		if channel != nil {
			channel <- response
			close(channel)
		}
	}
}

func (h *TCPClient) Call(ctx context.Context, input []Input, result *[]Output) error {
	if h.connection == nil {
		return fmt.Errorf("client not connected")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	channel := make(chan []byte)
	h.waitingResponsesMu.Lock()
	h.counter++
	msgID := h.counter
	h.waitingResponses[msgID] = channel
	h.waitingResponsesMu.Unlock()
	h.connection.Write(packets.Create(body, 0, msgID))
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
			h.waitingResponsesMu.Lock()
			delete(h.waitingResponses, msgID)
			close(channel)
			h.waitingResponsesMu.Unlock()
			return ctx.Err()
		}
	}
}

func (h *TCPClient) CallSingle(ctx context.Context, method string, params any, result any) error {
	return CallSingle(h, ctx, method, params, result)
}
