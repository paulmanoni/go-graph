package graph

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
)

// WebSocketManager manages WebSocket connections for GraphQL subscriptions.
// It handles connection lifecycle, authentication, and message routing.
type WebSocketManager struct {
	upgrader      websocket.Upgrader
	connections   sync.Map // map[string]*Connection
	schema        *graphql.Schema
	authFn        func(r *http.Request) (interface{}, error)
	pubsub        PubSub
	rootObjectFn  func(ctx context.Context, r *http.Request) map[string]interface{}
}

// Connection represents a single WebSocket connection.
type Connection struct {
	id            string
	ws            *websocket.Conn
	ctx           context.Context
	cancel        context.CancelFunc
	subscriptions map[string]context.CancelFunc // subscription ID -> cancel function
	mu            sync.RWMutex
	userDetails   interface{}
	rootValue     map[string]interface{}
	manager       *WebSocketManager
	messageChan   chan *WSMessage
	acknowledged  bool
	pingTicker    *time.Ticker
}

// WSMessage represents a GraphQL WebSocket Protocol message.
// This follows the graphql-ws protocol specification.
type WSMessage struct {
	ID      string                 `json:"id,omitempty"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// GraphQL WebSocket Protocol message types
// Supports both graphql-ws (new) and subscriptions-transport-ws (legacy) protocols
const (
	// Client -> Server (graphql-ws)
	MessageTypeConnectionInit = "connection_init"
	MessageTypeSubscribe      = "subscribe"
	MessageTypePing           = "ping"

	// Client -> Server (subscriptions-transport-ws - legacy)
	MessageTypeStart = "start" // Legacy equivalent of "subscribe"
	MessageTypeStop  = "stop"  // Legacy equivalent of "complete"

	// Server -> Client (graphql-ws)
	MessageTypeConnectionAck = "connection_ack"
	MessageTypeNext          = "next"
	MessageTypeError         = "error"
	MessageTypeComplete      = "complete"
	MessageTypePong          = "pong"

	// Server -> Client (subscriptions-transport-ws - legacy)
	MessageTypeData             = "data"              // Legacy equivalent of "next"
	MessageTypeConnectionError  = "connection_error"  // Legacy connection error
	MessageTypeConnectionKeepAlive = "ka"             // Legacy keep-alive
)

// WebSocketParams configures the WebSocket handler for subscriptions.
type WebSocketParams struct {
	// Schema: The GraphQL schema with subscription fields
	Schema *graphql.Schema

	// PubSub: The PubSub system for event distribution
	PubSub PubSub

	// CheckOrigin: Function to check WebSocket upgrade origin
	// If nil, all origins are allowed (development only!)
	CheckOrigin func(r *http.Request) bool

	// AuthFn: Authentication function to extract user details from request
	// Called during connection_init phase
	AuthFn func(r *http.Request) (interface{}, error)

	// RootObjectFn: Custom function to set up root object for each connection
	// Similar to HTTP handler's RootObjectFn
	RootObjectFn func(ctx context.Context, r *http.Request) map[string]interface{}

	// PingInterval: Interval for sending ping messages (default: 30 seconds)
	// Set to 0 to disable automatic pinging
	PingInterval time.Duration

	// ConnectionTimeout: Timeout for connection_init message (default: 10 seconds)
	ConnectionTimeout time.Duration
}

// NewWebSocketHandler creates an HTTP handler for WebSocket connections.
// This handler upgrades HTTP connections to WebSocket and manages GraphQL subscriptions.
//
// Example:
//
//	params := graph.WebSocketParams{
//	    Schema:      schema,
//	    PubSub:      pubsub,
//	    CheckOrigin: func(r *http.Request) bool {
//	        origin := r.Header.Get("Origin")
//	        return origin == "https://example.com"
//	    },
//	    AuthFn: func(r *http.Request) (interface{}, error) {
//	        token := ExtractBearerToken(r)
//	        return validateToken(token)
//	    },
//	}
//
//	http.Handle("/graphql", graph.NewWebSocketHandler(params))
func NewWebSocketHandler(params WebSocketParams) http.HandlerFunc {
	// Set defaults
	if params.PingInterval == 0 {
		params.PingInterval = 30 * time.Second
	}
	if params.ConnectionTimeout == 0 {
		params.ConnectionTimeout = 10 * time.Second
	}
	if params.CheckOrigin == nil {
		// Allow all origins (development only!)
		params.CheckOrigin = func(r *http.Request) bool { return true }
	}

	mgr := &WebSocketManager{
		upgrader: websocket.Upgrader{
			CheckOrigin:     params.CheckOrigin,
			Subprotocols:    []string{"graphql-transport-ws", "graphql-ws"},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		schema:       params.Schema,
		authFn:       params.AuthFn,
		pubsub:       params.PubSub,
		rootObjectFn: params.RootObjectFn,
	}

	return mgr.HandleWebSocket
}

// HandleWebSocket upgrades HTTP connections to WebSocket and manages the connection lifecycle.
func (m *WebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if it's a WebSocket upgrade request
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "Expected WebSocket upgrade", http.StatusBadRequest)
		return
	}

	// Upgrade connection
	ws, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Create connection context
	ctx, cancel := context.WithCancel(r.Context())

	conn := &Connection{
		id:            uuid.New().String(),
		ws:            ws,
		ctx:           ctx,
		cancel:        cancel,
		subscriptions: make(map[string]context.CancelFunc),
		manager:       m,
		messageChan:   make(chan *WSMessage, 100),
		rootValue:     make(map[string]interface{}),
	}

	// Set up root value if RootObjectFn is provided
	if m.rootObjectFn != nil {
		conn.rootValue = m.rootObjectFn(ctx, r)
	}

	// Store connection
	m.connections.Store(conn.id, conn)

	// Start connection handler
	conn.start()

	// Clean up when done
	conn.cleanup()
	m.connections.Delete(conn.id)
}

// start begins handling messages for this connection.
func (c *Connection) start() {
	// Start write pump (handles outgoing messages)
	go c.writePump()

	// Start read pump (handles incoming messages) - this blocks until connection closes
	c.readPump()
}

// readPump reads messages from the WebSocket connection.
func (c *Connection) readPump() {
	defer c.cancel()

	for {
		var msg WSMessage
		if err := c.ws.ReadJSON(&msg); err != nil {
			return
		}

		// Handle message
		c.handleMessage(&msg)
	}
}

// writePump sends messages to the WebSocket connection.
func (c *Connection) writePump() {
	defer c.cancel()

	for {
		select {
		case msg := <-c.messageChan:
			if err := c.ws.WriteJSON(msg); err != nil {
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// handleMessage processes incoming WebSocket messages.
// Supports both graphql-ws and subscriptions-transport-ws (legacy) protocols.
func (c *Connection) handleMessage(msg *WSMessage) {
	switch msg.Type {
	case MessageTypeConnectionInit:
		c.handleConnectionInit(msg)

	case MessageTypeSubscribe, MessageTypeStart: // Support both new and legacy
		if !c.acknowledged {
			c.sendError("", "Connection not initialized")
			return
		}
		c.handleSubscribe(msg)

	case MessageTypeComplete, MessageTypeStop: // Support both new and legacy
		if !c.acknowledged {
			return
		}
		c.handleComplete(msg)

	case MessageTypePing:
		c.sendMessage(&WSMessage{Type: MessageTypePong})

	default:
		c.sendError(msg.ID, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

// handleConnectionInit processes connection initialization.
func (c *Connection) handleConnectionInit(msg *WSMessage) {
	if c.acknowledged {
		c.sendError("", "Connection already initialized")
		return
	}

	// Extract connection params (could include auth token)
	var authToken string
	if msg.Payload != nil {
		if token, ok := msg.Payload["authorization"].(string); ok {
			authToken = token
		} else if token, ok := msg.Payload["Authorization"].(string); ok {
			authToken = token
		}
	}

	// Authenticate if authFn is provided
	if c.manager.authFn != nil && authToken != "" {
		// Create a fake request with the auth token for authFn
		fakeReq := &http.Request{
			Header: http.Header{
				"Authorization": []string{authToken},
			},
		}

		userDetails, err := c.manager.authFn(fakeReq)
		if err != nil {
			c.sendError("", fmt.Sprintf("Authentication failed: %s", err.Error()))
			c.cancel()
			return
		}
		c.userDetails = userDetails
		c.rootValue["details"] = userDetails
	}

	// Mark as acknowledged
	c.acknowledged = true

	// Send connection_ack
	c.sendMessage(&WSMessage{Type: MessageTypeConnectionAck})

	// Start keep-alive ticker (supports both protocols)
	c.pingTicker = time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-c.pingTicker.C:
				// Send both ping (graphql-ws) and ka (legacy) for compatibility
				c.sendMessage(&WSMessage{Type: MessageTypePing})
				c.sendMessage(&WSMessage{Type: MessageTypeConnectionKeepAlive})
			case <-c.ctx.Done():
				c.pingTicker.Stop()
				return
			}
		}
	}()
}

// handleSubscribe starts a new subscription.
func (c *Connection) handleSubscribe(msg *WSMessage) {
	if msg.ID == "" {
		c.sendError("", "Subscription ID required")
		return
	}

	if msg.Payload == nil {
		c.sendError(msg.ID, "Subscription payload required")
		return
	}

	// Extract query and variables
	query, ok := msg.Payload["query"].(string)
	if !ok {
		c.sendError(msg.ID, "Query required in subscription payload")
		return
	}

	variables, _ := msg.Payload["variables"].(map[string]interface{})

	// Create subscription context (can be canceled independently)
	subCtx, cancel := context.WithCancel(c.ctx)

	// Store cancel function
	c.mu.Lock()
	c.subscriptions[msg.ID] = cancel
	c.mu.Unlock()

	// Execute subscription
	go c.executeSubscription(subCtx, msg.ID, query, variables)
}

// executeSubscription runs the GraphQL subscription and sends events to the client.
func (c *Connection) executeSubscription(ctx context.Context, subscriptionID, query string, variables map[string]interface{}) {
	// Execute GraphQL subscription
	params := graphql.Params{
		Schema:         *c.manager.schema,
		RequestString:  query,
		VariableValues: variables,
		RootObject:     c.rootValue,
		Context:        ctx,
	}

	resultChannel := graphql.Subscribe(params)

	// Listen for results
	for {
		select {
		case result, ok := <-resultChannel:
			if !ok {
				// Channel closed - subscription complete
				c.sendComplete(subscriptionID)
				return
			}

			// Check for errors
			if len(result.Errors) > 0 {
				for _, err := range result.Errors {
					c.sendError(subscriptionID, err.Error())
				}
				continue
			}

			// Send event data to client
			if result.Data != nil {
				c.sendNext(subscriptionID, result.Data)
			}

		case <-ctx.Done():
			// Subscription canceled
			c.sendComplete(subscriptionID)
			return
		}
	}
}

// handleComplete stops an active subscription.
func (c *Connection) handleComplete(msg *WSMessage) {
	if msg.ID == "" {
		return
	}

	c.mu.Lock()
	if cancel, exists := c.subscriptions[msg.ID]; exists {
		cancel()
		delete(c.subscriptions, msg.ID)
	}
	c.mu.Unlock()
}

// sendMessage sends a message to the client.
func (c *Connection) sendMessage(msg *WSMessage) {
	select {
	case c.messageChan <- msg:
	case <-c.ctx.Done():
	}
}

// sendNext sends a subscription event to the client.
// Sends both "next" (graphql-ws) and "data" (legacy) for compatibility.
func (c *Connection) sendNext(subscriptionID string, data interface{}) {
	payload := map[string]interface{}{
		"data": data,
	}

	// Send using new protocol (graphql-ws)
	c.sendMessage(&WSMessage{
		ID:      subscriptionID,
		Type:    MessageTypeNext,
		Payload: payload,
	})

	// Also send using legacy protocol (subscriptions-transport-ws) for older clients
	c.sendMessage(&WSMessage{
		ID:      subscriptionID,
		Type:    MessageTypeData,
		Payload: payload,
	})
}

// sendError sends an error message to the client.
func (c *Connection) sendError(subscriptionID, message string) {
	payload := map[string]interface{}{
		"errors": []map[string]interface{}{
			{"message": message},
		},
	}

	c.sendMessage(&WSMessage{
		ID:      subscriptionID,
		Type:    MessageTypeError,
		Payload: payload,
	})
}

// sendComplete sends a subscription complete message to the client.
func (c *Connection) sendComplete(subscriptionID string) {
	c.sendMessage(&WSMessage{
		ID:   subscriptionID,
		Type: MessageTypeComplete,
	})
}

// cleanup closes the connection and cancels all subscriptions.
func (c *Connection) cleanup() {
	// Cancel all subscriptions
	c.mu.Lock()
	for _, cancel := range c.subscriptions {
		cancel()
	}
	c.subscriptions = make(map[string]context.CancelFunc)
	c.mu.Unlock()

	// Stop ping ticker
	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}

	// Close message channel
	close(c.messageChan)

	// Close WebSocket connection
	c.ws.Close()
}

// CloseAllConnections closes all active WebSocket connections.
// This is useful for graceful shutdown.
func (m *WebSocketManager) CloseAllConnections() {
	m.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*Connection); ok {
			conn.cancel()
		}
		return true
	})
}