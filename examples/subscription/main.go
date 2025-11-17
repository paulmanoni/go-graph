package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	graph "github.com/paulmanoni/go-graph"
)

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	ChannelID string    `json:"channelID"`
	Timestamp time.Time `json:"timestamp"`
}

// User represents a user in the system
type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

// UserStatusEvent represents a user status change event
type UserStatusEvent struct {
	UserID    string    `json:"userID"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// In-memory data store (for demo purposes)
var (
	users = map[string]*User{
		"1": {ID: "1", Name: "Alice", Email: "alice@example.com", Status: "online"},
		"2": {ID: "2", Name: "Bob", Email: "bob@example.com", Status: "offline"},
	}
	messages = []*Message{}
)

func main() {
	// Initialize PubSub system
	pubsub := graph.NewInMemoryPubSub()
	defer pubsub.Close()

	// Build GraphQL handler with subscriptions
	handler := graph.NewHTTP(&graph.GraphContext{
		SchemaParams: &graph.SchemaBuilderParams{
			QueryFields: []graph.QueryField{
				getUserQuery(),
				getMessagesQuery(),
			},
			MutationFields: []graph.MutationField{
				sendMessageMutation(pubsub),
				updateUserStatusMutation(pubsub),
			},
			SubscriptionFields: []graph.SubscriptionField{
				messageSubscription(pubsub),
				userStatusSubscription(pubsub),
			},
		},
		PubSub:              pubsub,
		EnableSubscriptions: true,
		Playground:          true,
		DEBUG:               true,
	})

	// Set up HTTP server
	// Handle both /graphql and /subscriptions endpoints for compatibility
	http.Handle("/graphql", handler)
	http.Handle("/subscriptions", handler) // Some GraphQL clients expect subscriptions here

	// Start background task to simulate activity
	go simulateActivity(pubsub)

	fmt.Println("🚀 GraphQL server with subscriptions running on http://localhost:8080/graphql")
	fmt.Println("📝 GraphQL Playground available at http://localhost:8080/graphql")
	fmt.Println("\n🔔 Try these subscriptions in the Playground:")
	fmt.Println("\nSubscribe to messages:")
	fmt.Println(`subscription {
  messageAdded(channelID: "general") {
    id
    content
    author
    timestamp
  }
}`)
	fmt.Println("\nSubscribe to user status changes:")
	fmt.Println(`subscription {
  userStatusChanged {
    userID
    status
    timestamp
  }
}`)
	fmt.Println("\nSend a message (in another tab):")
	fmt.Println(`mutation {
  sendMessage(input: { channelID: "general", content: "Hello!", author: "Alice" }) {
    id
    content
    author
  }
}`)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Query: Get user by ID
func getUserQuery() graph.QueryField {
	return graph.NewResolver[User]("user").
		WithDescription("Get a user by ID").
		WithArgs(graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "User ID",
			},
		}).
		WithResolver(func(p graph.ResolveParams) (*User, error) {
			id, _ := graph.GetArgString(p, "id")
			if user, ok := users[id]; ok {
				return user, nil
			}
			return nil, fmt.Errorf("user not found")
		}).
		BuildQuery()
}

// Query: Get messages for a channel
func getMessagesQuery() graph.QueryField {
	return graph.NewResolver[[]*Message]("messages").
		WithDescription("Get messages for a channel").
		WithArgs(graphql.FieldConfigArgument{
			"channelID": &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Channel ID",
			},
		}).
		WithResolver(func(p graph.ResolveParams) (*[]*Message, error) {
			channelID, _ := graph.GetArgString(p, "channelID")

			// Filter messages by channel
			var filtered []*Message
			for _, msg := range messages {
				if msg.ChannelID == channelID {
					filtered = append(filtered, msg)
				}
			}

			return &filtered, nil
		}).
		BuildQuery()
}

// Mutation: Send a message
func sendMessageMutation(pubsub graph.PubSub) graph.MutationField {
	type SendMessageInput struct {
		ChannelID string `json:"channelID" graphql:"channelID,required"`
		Content   string `json:"content" graphql:"content,required"`
		Author    string `json:"author" graphql:"author,required"`
	}

	return graph.NewResolver[Message]("sendMessage").
		WithDescription("Send a message to a channel").
		WithInputObject(SendMessageInput{}).
		WithResolver(func(p graph.ResolveParams) (*Message, error) {
			var input SendMessageInput
			if err := graph.GetArg(p, "input", &input); err != nil {
				return nil, err
			}

			// Create message
			msg := &Message{
				ID:        uuid.New().String(),
				Content:   input.Content,
				Author:    input.Author,
				ChannelID: input.ChannelID,
				Timestamp: time.Now(),
			}

			// Store message
			messages = append(messages, msg)

			// Publish to subscribers
			ctx := context.Background()
			if err := pubsub.Publish(ctx, "messages:"+input.ChannelID, msg); err != nil {
				log.Printf("Failed to publish message: %v", err)
			}

			return msg, nil
		}).
		BuildMutation()
}

// Mutation: Update user status
func updateUserStatusMutation(pubsub graph.PubSub) graph.MutationField {
	type UpdateStatusInput struct {
		UserID string `json:"userID" graphql:"userID,required"`
		Status string `json:"status" graphql:"status,required"`
	}

	return graph.NewResolver[User]("updateUserStatus").
		WithDescription("Update a user's status").
		WithInputObject(UpdateStatusInput{}).
		WithResolver(func(p graph.ResolveParams) (*User, error) {
			var input UpdateStatusInput
			if err := graph.GetArg(p, "input", &input); err != nil {
				return nil, err
			}

			// Update user
			if user, ok := users[input.UserID]; ok {
				user.Status = input.Status

				// Create status event
				event := &UserStatusEvent{
					UserID:    input.UserID,
					Status:    input.Status,
					Timestamp: time.Now(),
				}

				// Publish to subscribers
				ctx := context.Background()
				if err := pubsub.Publish(ctx, "user_status", event); err != nil {
					log.Printf("Failed to publish status change: %v", err)
				}

				return user, nil
			}

			return nil, fmt.Errorf("user not found")
		}).
		BuildMutation()
}

// Subscription: Subscribe to messages in a channel
func messageSubscription(pubsub graph.PubSub) graph.SubscriptionField {
	return graph.NewSubscription[Message]("messageAdded").
		WithDescription("Subscribe to new messages in a channel").
		WithArgs(graphql.FieldConfigArgument{
			"channelID": &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Channel ID to subscribe to",
			},
		}).
		WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *Message, error) {
			channelID, _ := graph.GetArgString(p, "channelID")

			// Create output channel
			events := make(chan *Message, 10)

			// Subscribe to PubSub topic
			subscription := pubsub.Subscribe(ctx, "messages:"+channelID)

			// Start goroutine to forward messages
			go func() {
				defer close(events)
				for msg := range subscription {
					var message Message
					if err := json.Unmarshal(msg.Data, &message); err == nil {
						events <- &message
					}
				}
			}()

			return events, nil
		}).
		BuildSubscription()
}

// Subscription: Subscribe to user status changes
func userStatusSubscription(pubsub graph.PubSub) graph.SubscriptionField {
	return graph.NewSubscription[UserStatusEvent]("userStatusChanged").
		WithDescription("Subscribe to user status changes").
		WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *UserStatusEvent, error) {
			// Create output channel
			events := make(chan *UserStatusEvent, 10)

			// Subscribe to PubSub topic
			subscription := pubsub.Subscribe(ctx, "user_status")

			// Start goroutine to forward events
			go func() {
				defer close(events)
				for msg := range subscription {
					var event UserStatusEvent
					if err := json.Unmarshal(msg.Data, &event); err == nil {
						events <- &event
					}
				}
			}()

			return events, nil
		}).
		BuildSubscription()
}

// simulateActivity generates periodic events for demonstration
func simulateActivity(pubsub graph.PubSub) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	messageCount := 0
	authors := []string{"Alice", "Bob", "System"}
	statuses := []string{"online", "away", "busy"}

	for range ticker.C {
		ctx := context.Background()

		// Randomly send a message
		if messageCount%2 == 0 {
			msg := &Message{
				ID:        uuid.New().String(),
				Content:   fmt.Sprintf("Automated message #%d", messageCount),
				Author:    authors[messageCount%len(authors)],
				ChannelID: "general",
				Timestamp: time.Now(),
			}
			messages = append(messages, msg)
			pubsub.Publish(ctx, "messages:general", msg)
			log.Printf("📨 Sent automated message: %s", msg.Content)
		} else {
			// Randomly update a user status
			userID := fmt.Sprintf("%d", (messageCount%2)+1)
			status := statuses[messageCount%len(statuses)]

			if user, ok := users[userID]; ok {
				user.Status = status
				event := &UserStatusEvent{
					UserID:    userID,
					Status:    status,
					Timestamp: time.Now(),
				}
				pubsub.Publish(ctx, "user_status", event)
				log.Printf("👤 Updated user %s status to: %s", user.Name, status)
			}
		}

		messageCount++
	}
}