package main

import (
	"fmt"
	"github.com/tarantool/go-tarantool"
	"time"
)

const (
	CONNECT_BIND_ADDR   = "127.0.0.1:60501"
	CONNECT_RETRY_COUNT = 10
)

// Database config

type PushDbConfig struct {
	Timeout            time.Duration
	Reconnect          time.Duration
	PingIntervalMillis Millitime
	MaxReconnects      uint
	Bind               string
}

func (db PushDbConfig) Connect(addr string) (*tarantool.Connection, error) {
	fmt.Printf("Database: addr = %q, timeout = %s, reconnect = %s, max = %d\n",
		addr, db.Timeout, db.Reconnect, db.MaxReconnects)

	opts := tarantool.Opts{
		Timeout:       db.Timeout,
		Reconnect:     db.Reconnect,
		MaxReconnects: db.MaxReconnects}

	fmt.Println("Access control:", "guest")

	var lastErr error = nil
	lastDelay := 250 * time.Millisecond
	for i := 0; i < CONNECT_RETRY_COUNT; i++ {
		client, err := tarantool.Connect(addr, opts)
		if err == nil {
			fmt.Printf("Connected to %q\n", addr)

			if db.PingIntervalMillis != 0 {
				fmt.Printf("Starting a thread to ping the db every %d seconds\n", db.PingIntervalMillis/TIME_MS_1_SECOND)
				go db.pingWorker(client)
			}

			return client, err
		}

		lastErr = err
		terr, ok := err.(tarantool.ClientError)
		if !ok || terr.Code != tarantool.ErrConnectionNotReady {
			return client, err
		}

		fmt.Printf("Waiting %s and will try connecting again...\n", lastDelay)
		time.Sleep(lastDelay)
		lastDelay = lastDelay * 2
	}
	return nil, lastErr
}

func (db PushDbConfig) pingWorker(client *tarantool.Connection) {
	for {
		select {
		case <-time.After(time.Duration(db.PingIntervalMillis) * time.Millisecond):
			fmt.Printf("*** Ping-ing the server ***\n")

			resp, err := client.Ping()
			if err != nil {
				fmt.Printf("Ping: %v, %v\n", resp, err)
			}

			if err != nil {
				terr, ok := err.(tarantool.ClientError)
				if ok && terr.Code == tarantool.ErrConnectionClosed {

					msg := fmt.Sprintf("Tarantool connection has been closed")
					panic(msg)
				}
			}
		}
	}
}

func NewDbConfig() (*PushDbConfig, error) {
	db := PushDbConfig{
		Timeout:            5000 * time.Millisecond,
		Reconnect:          1 * time.Second,
		PingIntervalMillis: 5 * TIME_MS_1_SECOND,
		MaxReconnects:      10,
		Bind:               CONNECT_BIND_ADDR}

	return &db, nil
}
