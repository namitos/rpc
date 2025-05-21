package rpc

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"testing"
	"time"
)

type testData1 struct {
	ZZZZ int64 `validate:"required"`
	Time int64
}

type testData struct {
	Td *testData1
	testData1
	Time int64
}

func TestSchema(t *testing.T) {
	RPCMethods := &Server{}
	RPCMethods.Set("test", func(td testData) testData {
		log.Println("received", td)
		time.Sleep(1 * time.Second)
		return td
	})
	methodSchema, err := RPCMethods.GetMethodSchema("test")
	methodSchemaJSON, _ := json.MarshalIndent(methodSchema, "  ", "  ")
	log.Println(string(methodSchemaJSON), err)
}

// TODO: make real test
func TestRPC(t *testing.T) {
	RPCMethods := &Server{}
	RPCMethods.Set("test", func() *testData {
		now := time.Now().UnixMilli()
		time.Sleep(time.Second)
		log.Println("received", now)
		return &testData{Time: now}
	})
	//RPCMethods.Set("test", func(td *testData) *testData {
	//	log.Println("received", td)
	//	time.Sleep(time.Second)
	//	return td
	//})
	//RPCMethods.Set("test", func(td testData) testData {
	//	log.Println("received", td)
	//	time.Sleep(15 * time.Second)
	//	return td
	//})

	go func() {
		//time.AfterFunc(time.Second*10, func() {
		//	err := RPCMethods.CloseTCP()
		//	log.Println("closed", err)
		//})
		err := RPCMethods.ListenTCP("8001")
		if err != nil {
			log.Println(err)
		}
	}()

	time.AfterFunc(time.Millisecond, func() {
		client := NewTCPClient("127.0.0.1:8001")

		time.AfterFunc(time.Millisecond, func() {
			wg := &sync.WaitGroup{}
			wg.Add(5)
			for i := 0; i < 5; i++ {
				go func(i int) {
					defer wg.Done()
					result := &testData{}
					t := time.Now().UnixNano()
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					err := client.Call(ctx, []Input{{
						Method: "test",
						Params: map[string]int64{"Time": t},
					}}, &[]Output{{Result: result}})
					log.Println(err, t, result.Time, i)
				}(i)
			}
			wg.Wait()
			log.Println("end")
		})
	})

	select {}
}

func TestRPCError(t *testing.T) {
	RPCMethods := &Server{}
	RPCMethods.Set("testError", func() (any, error) {
		return nil, &OutputError{
			Code:    123,
			Message: "errrrrr",
		}
	})
	go func() {
		err := RPCMethods.ListenTCP("8001")
		if err != nil {
			log.Println(err)
		}
	}()

	time.AfterFunc(time.Millisecond, func() {
		client := NewTCPClient("127.0.0.1:8001")

		time.AfterFunc(time.Millisecond, func() {
			err := client.CallSingle(context.Background(), "testError", nil, nil)
			log.Println(err)
		})
	})

	select {}
}
