package ws

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/suhostersky/gate-ws/types"
)

const (
	WEBSOCKET_TRADE_MAINNET = "wss://fx-ws.gateio.ws/v4/ws/usdt"
	WEBSOCKET_SPOT_MAINNET  = "wss://api.gateio.ws/ws/v4/"

	FUTURES_LOGIN       = "futures.login"
	FUTURES_PING        = "futures.ping"
	FUTURES_PONG        = "futures.pong"
	FUTURES_ORDER_PLACE = "futures.order_place"

	SPOT_LOGIN       = "spot.login"
	SPOT_PING        = "spot.ping"
	SPOT_PONG        = "spot.pong"
	SPOT_ORDER_PLACE = "spot.order_place"
)

type Market string

const (
	Futures Market = "futures"
	Spot    Market = "spot"
)

func (m Market) Order() string {
	switch m {
	case Futures:
		return FUTURES_ORDER_PLACE
	case Spot:
		return SPOT_ORDER_PLACE
	default:
		return ""
	}
}

func (m Market) Login() string {
	switch m {
	case Futures:
		return FUTURES_LOGIN
	case Spot:
		return SPOT_LOGIN
	default:
		return ""
	}
}

func (m Market) Ping() string {
	switch m {
	case Futures:
		return FUTURES_PING
	case Spot:
		return SPOT_PING
	default:
		return ""
	}
}

type MessageHandler func(message string) error

func (b *WebSocket) handleIncomingMessages() {
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
			_, message, err := b.conn.ReadMessage()
			if err != nil {
				b.isConnected = false
				log.Println("Error reading:", err)
				return
			}

			if b.onMessage != nil {
				err := b.onMessage(string(message))
				if err != nil {
					log.Println("Error handling message:", err)
					return
				}
			}
		}
	}
}

func (b *WebSocket) monitorConnection() {
	ticker := time.NewTicker(time.Second * 5) // Check every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			if !b.isConnected && b.ctx.Err() == nil { // Check if disconnected and context not done
				log.Println("Attempting to reconnect...")
				err := b.Connect() // Example, adjust parameters as needed
				if err != nil {
					log.Println("Reconnection failed:")
				} else {
					b.isConnected = true
					go b.handleIncomingMessages() // Restart message handling
				}
			}
		}
	}
}

func (b *WebSocket) SetMessageHandler(handler MessageHandler) {
	b.onMessage = handler
}

type WebSocket struct {
	conn         *websocket.Conn
	url          string
	apiKey       string
	apiSecret    string
	maxAliveTime string
	pingInterval int
	onMessage    MessageHandler
	ctx          context.Context
	cancel       context.CancelFunc
	isConnected  bool
	mutex        sync.Mutex
	market       Market
}

type WebsocketOption func(*WebSocket)

func WithPingInterval(pingInterval int) WebsocketOption {
	return func(c *WebSocket) {
		c.pingInterval = pingInterval
	}
}

func WithMaxAliveTime(maxAliveTime string) WebsocketOption {
	return func(c *WebSocket) {
		c.maxAliveTime = maxAliveTime
	}
}

func NewGatePrivateWebSocket(market Market, url, apiKey, apiSecret string, handler MessageHandler, options ...WebsocketOption) *WebSocket {
	c := &WebSocket{
		url:          url,
		apiKey:       apiKey,
		apiSecret:    apiSecret,
		maxAliveTime: "",
		pingInterval: 20,
		onMessage:    handler,
		market:       market,
	}

	// Apply the provided options
	for _, opt := range options {
		opt(c)
	}

	return c
}

func (b *WebSocket) Connect() error {
	var err error
	b.conn, _, err = websocket.DefaultDialer.Dial(b.url, nil)
	if err != nil {
		return err
	}

	if err = b.sendAuth(); err != nil {
		return err
	}
	b.isConnected = true

	go b.handleIncomingMessages()

	b.ctx, b.cancel = context.WithCancel(context.Background())
	go ping(b)

	return nil
}

func ping(b *WebSocket) {
	if b.pingInterval <= 0 {
		log.Println("Ping interval is set to a non-positive value.")
		return
	}

	ticker := time.NewTicker(time.Duration(b.pingInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentTime := time.Now().Unix()
			pingMessage := types.ApiRequest{
				Time:    currentTime,
				Channel: b.market.Ping(),
			}

			if err := b.sendAsJson(pingMessage); err != nil {
				log.Println("Failed to send ping:", err)
				return
			}
		case <-b.ctx.Done():
			log.Println("Ping context closed, stopping ping.")
			return
		}
	}
}

func (b *WebSocket) Disconnect() error {
	b.cancel()
	b.isConnected = false
	return b.conn.Close()
}

func (b *WebSocket) sendAuth() error {
	// Get current Unix time in milliseconds
	ts := time.Now().Unix()
	requestId := fmt.Sprintf("%d-%d", time.Now().UnixMilli(), 1)

	authMessage := types.ApiRequest{
		Time:    ts,
		Channel: b.market.Login(),
		Event:   "api",
		Payload: types.ApiPayload{
			ApiKey:       b.apiKey,
			Signature:    getApiSignature(b.apiSecret, b.market.Login(), []byte(""), ts),
			Timestamp:    strconv.FormatInt(ts, 10),
			RequestId:    requestId,
			RequestParam: []byte(""),
		},
	}

	return b.sendAsJson(authMessage)
}

func getApiSignature(secret, channel string, requestParam []byte, ts int64) string {
	hash := hmac.New(sha512.New, []byte(secret))
	key := fmt.Sprintf("%s\n%s\n%s\n%d", "api", channel, string(requestParam), ts)
	hash.Write([]byte(key))
	return hex.EncodeToString(hash.Sum(nil))
}

func (b *WebSocket) PlaceOrder(params *types.OrderParam) error {
	orderParamBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}
	ts := time.Now().Unix()
	requestId := fmt.Sprintf("%d-%d", time.Now().UnixMilli(), 1)
	orderPlace := types.ApiRequest{
		Time:    ts,
		Channel: b.market.Order(),
		Event:   "api",
		Payload: types.ApiPayload{
			RequestId:    requestId,
			RequestParam: orderParamBytes,
		},
	}

	return b.sendAsJson(orderPlace)
}

func (b *WebSocket) sendAsJson(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return b.send(string(data))
}

func (b *WebSocket) send(message string) error {
	if b.conn == nil {
		return errors.New("websocket connection is nil")
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	err := b.conn.WriteMessage(websocket.TextMessage, []byte(message))
	return err
}
