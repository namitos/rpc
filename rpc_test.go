package rpc

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"
)

type testData struct {
	Time int64
}

//TODO: make real test
func TestRPC(t *testing.T) {
	RPCMethods := &Server{KeepAlive: true}
	RPCMethods.Set("test", func(td *testData) (*testData, error) {
		log.Println("received", td)
		time.Sleep(1 * time.Second)
		return td, nil
	})
	go func() {
		RPCMethods.ListenTCP("8001")
	}()

	time.AfterFunc(10000000, func() {
		client := NewTCPClientKeepAlive("127.0.0.1:8001")

		time.AfterFunc(10000000, func() {
			wg := &sync.WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func(i int) {
					defer wg.Done()
					result := &testData{}
					t := time.Now().UnixNano()
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					err := client.Call(ctx, &[]Input{{
						Method: "test",
						Params: map[string]int64{"Time": t},
					}}, &[]Output{{Result: result}})
					if err != nil {
						log.Println(err)
					}
					log.Println(err, t, result.Time, i)
				}(i)
			}
			wg.Wait()
			log.Println("end")
		})
	})

	select {}
}
