package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/urfave/cli/v2"
)

type Config struct {
	HTTPEndpoint      string
	WebSocketEndpoint string
}

type TestStats struct {
	TotalRequests    atomic.Int64
	SuccessRequests  atomic.Int64
	FailedRequests   atomic.Int64
	TotalConnections atomic.Int64
	ActiveConnections atomic.Int64
	TotalMessages    atomic.Int64
	StartTime        time.Time
}

func (s *TestStats) IncrementTotal() {
	s.TotalRequests.Add(1)
}

func (s *TestStats) IncrementSuccess() {
	s.SuccessRequests.Add(1)
}

func (s *TestStats) IncrementFailed() {
	s.FailedRequests.Add(1)
}

func (s *TestStats) IncrementConnections() {
	s.TotalConnections.Add(1)
	s.ActiveConnections.Add(1)
}

func (s *TestStats) DecrementConnections() {
	s.ActiveConnections.Add(-1)
}

func (s *TestStats) IncrementMessages() {
	s.TotalMessages.Add(1)
}

func (s *TestStats) PrintStats() {
	elapsed := time.Since(s.StartTime).Seconds()
	fmt.Println("\n========== Test Statistics ==========")
	fmt.Printf("Total Requests:       %d\n", s.TotalRequests.Load())
	fmt.Printf("Successful Requests:  %d\n", s.SuccessRequests.Load())
	fmt.Printf("Failed Requests:      %d\n", s.FailedRequests.Load())
	fmt.Printf("Total Connections:    %d\n", s.TotalConnections.Load())
	fmt.Printf("Active Connections:   %d\n", s.ActiveConnections.Load())
	fmt.Printf("Total Messages Sent:  %d\n", s.TotalMessages.Load())
	fmt.Printf("Test Duration:        %.2f seconds\n", elapsed)
	if elapsed > 0 {
		fmt.Printf("Requests/second:      %.2f\n", float64(s.TotalRequests.Load())/elapsed)
		fmt.Printf("Messages/second:      %.2f\n", float64(s.TotalMessages.Load())/elapsed)
	}
	fmt.Println("=====================================")
}

var globalStats = &TestStats{
	StartTime: time.Now(),
}

func main() {
	app := &cli.App{
		Name:  "stress-test",
		Usage: "Stress testing tool for IM system",
		Commands: []*cli.Command{
			{
				Name:  "interactive",
				Usage: "Run interactive stress test",
				Action: func(c *cli.Context) error {
					return runInteractiveMode()
				},
			},
			{
				Name:  "scenario",
				Usage: "Run predefined stress test scenarios",
				Subcommands: []*cli.Command{
					{
						Name:  "user-creation",
						Usage: "Test user creation at scale",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "count",
								Usage: "Number of users to create",
								Value: 100,
							},
							&cli.StringFlag{
								Name:  "http",
								Usage: "HTTP endpoint",
								Value: "http://localhost:9503",
							},
						},
						Action: func(c *cli.Context) error {
							config := &Config{
								HTTPEndpoint: c.String("http"),
							}
							return runUserCreationTest(c.Context, config, c.Int("count"))
						},
					},
					{
						Name:  "friend-requests",
						Usage: "Test friend request operations",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "users",
								Usage: "Number of users",
								Value: 50,
							},
							&cli.IntFlag{
								Name:  "requests",
								Usage: "Number of friend requests per user",
								Value: 10,
							},
							&cli.StringFlag{
								Name:  "http",
								Usage: "HTTP endpoint",
								Value: "http://localhost:9503",
							},
						},
						Action: func(c *cli.Context) error {
							config := &Config{
								HTTPEndpoint: c.String("http"),
							}
							return runFriendRequestTest(c.Context, config, c.Int("users"), c.Int("requests"))
						},
					},
					{
						Name:  "group-chat",
						Usage: "Test group chat messaging",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "groups",
								Usage: "Number of groups",
								Value: 10,
							},
							&cli.IntFlag{
								Name:  "members",
								Usage: "Number of members per group",
								Value: 20,
							},
							&cli.IntFlag{
								Name:  "messages",
								Usage: "Number of messages per member",
								Value: 100,
							},
							&cli.StringFlag{
								Name:  "http",
								Usage: "HTTP endpoint",
								Value: "http://localhost:9503",
							},
							&cli.StringFlag{
								Name:  "ws",
								Usage: "WebSocket endpoint",
								Value: "ws://localhost:9501/wss",
							},
						},
						Action: func(c *cli.Context) error {
							config := &Config{
								HTTPEndpoint:      c.String("http"),
								WebSocketEndpoint: c.String("ws"),
							}
							return runGroupChatTest(c.Context, config, c.Int("groups"), c.Int("members"), c.Int("messages"))
						},
					},
					{
						Name:  "mixed-operations",
						Usage: "Run mixed operations stress test",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "duration",
								Usage: "Test duration in seconds",
								Value: 60,
							},
							&cli.IntFlag{
								Name:  "concurrency",
								Usage: "Number of concurrent users",
								Value: 50,
							},
							&cli.StringFlag{
								Name:  "http",
								Usage: "HTTP endpoint",
								Value: "http://localhost:9503",
							},
							&cli.StringFlag{
								Name:  "ws",
								Usage: "WebSocket endpoint",
								Value: "ws://localhost:9501/wss",
							},
						},
						Action: func(c *cli.Context) error {
							config := &Config{
								HTTPEndpoint:      c.String("http"),
								WebSocketEndpoint: c.String("ws"),
							}
							return runMixedOperationsTest(c.Context, config, c.Int("duration"), c.Int("concurrency"))
						},
					},
				},
			},
			{
				Name:  "cleanup",
				Usage: "Cleanup test data and reset database",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "Config file path",
						Value: "./config.yaml",
					},
				},
				Action: func(c *cli.Context) error {
					return runCleanup(c.String("config"))
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func runInteractiveMode() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("========== IM Stress Test Tool ==========")
	fmt.Println()

	// Get HTTP endpoint
	fmt.Print("Enter HTTP endpoint (default: http://localhost:9503): ")
	httpEndpoint, _ := reader.ReadString('\n')
	httpEndpoint = strings.TrimSpace(httpEndpoint)
	if httpEndpoint == "" {
		httpEndpoint = "http://localhost:9503"
	}

	// Get WebSocket endpoint
	fmt.Print("Enter WebSocket endpoint (default: ws://localhost:9501/wss): ")
	wsEndpoint, _ := reader.ReadString('\n')
	wsEndpoint = strings.TrimSpace(wsEndpoint)
	if wsEndpoint == "" {
		wsEndpoint = "ws://localhost:9501/wss"
	}

	config := &Config{
		HTTPEndpoint:      httpEndpoint,
		WebSocketEndpoint: wsEndpoint,
	}

	fmt.Println()
	fmt.Println("Select test scenario:")
	fmt.Println("1. User Creation Test")
	fmt.Println("2. Friend Request Test")
	fmt.Println("3. Group Chat Test")
	fmt.Println("4. Mixed Operations Test")
	fmt.Println("5. Exit")
	fmt.Println()
	fmt.Print("Enter choice: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	ctx := context.Background()

	switch choice {
	case "1":
		fmt.Print("Enter number of users to create (default: 100): ")
		var count int
		fmt.Fscanln(reader, &count)
		if count == 0 {
			count = 100
		}
		return runUserCreationTest(ctx, config, count)
	case "2":
		fmt.Print("Enter number of users (default: 50): ")
		var users int
		fmt.Fscanln(reader, &users)
		if users == 0 {
			users = 50
		}
		fmt.Print("Enter number of friend requests per user (default: 10): ")
		var requests int
		fmt.Fscanln(reader, &requests)
		if requests == 0 {
			requests = 10
		}
		return runFriendRequestTest(ctx, config, users, requests)
	case "3":
		fmt.Print("Enter number of groups (default: 10): ")
		var groups int
		fmt.Fscanln(reader, &groups)
		if groups == 0 {
			groups = 10
		}
		fmt.Print("Enter number of members per group (default: 20): ")
		var members int
		fmt.Fscanln(reader, &members)
		if members == 0 {
			members = 20
		}
		fmt.Print("Enter number of messages per member (default: 100): ")
		var messages int
		fmt.Fscanln(reader, &messages)
		if messages == 0 {
			messages = 100
		}
		return runGroupChatTest(ctx, config, groups, members, messages)
	case "4":
		fmt.Print("Enter test duration in seconds (default: 60): ")
		var duration int
		fmt.Fscanln(reader, &duration)
		if duration == 0 {
			duration = 60
		}
		fmt.Print("Enter number of concurrent users (default: 50): ")
		var concurrency int
		fmt.Fscanln(reader, &concurrency)
		if concurrency == 0 {
			concurrency = 50
		}
		return runMixedOperationsTest(ctx, config, duration, concurrency)
	case "5":
		fmt.Println("Exiting...")
		return nil
	default:
		return fmt.Errorf("invalid choice")
	}
}

func runUserCreationTest(ctx context.Context, config *Config, count int) error {
	fmt.Printf("\n========== User Creation Test ==========\n")
	fmt.Printf("Creating %d users...\n", count)
	globalStats.StartTime = time.Now()

	client := NewHTTPClient(config.HTTPEndpoint)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 50) // Limit concurrent requests

	for i := 0; i < count; i++ {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			username := fmt.Sprintf("stress_user_%d_%d", time.Now().Unix(), index)
			password := "Test@123456"

			err := client.Register(ctx, username, password)
			globalStats.IncrementTotal()
			
			if err != nil {
				globalStats.IncrementFailed()
				log.Printf("Failed to create user %s: %v", username, err)
			} else {
				globalStats.IncrementSuccess()
			}
		}(i)
	}

	wg.Wait()
	globalStats.PrintStats()
	
	return nil
}

func runFriendRequestTest(ctx context.Context, config *Config, userCount int, requestsPerUser int) error {
	fmt.Printf("\n========== Friend Request Test ==========\n")
	fmt.Printf("Creating %d users and sending %d friend requests each...\n", userCount, requestsPerUser)
	globalStats.StartTime = time.Now()

	client := NewHTTPClient(config.HTTPEndpoint)

	// First, create users
	users := make([]*User, 0, userCount)
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 50)

	fmt.Println("Creating users...")
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			username := fmt.Sprintf("stress_friend_%d_%d", time.Now().Unix(), index)
			password := "Test@123456"

			user, err := client.RegisterAndLogin(ctx, username, password)
			globalStats.IncrementTotal()
			
			if err != nil {
				globalStats.IncrementFailed()
				log.Printf("Failed to create/login user %s: %v", username, err)
			} else {
				globalStats.IncrementSuccess()
				mu.Lock()
				users = append(users, user)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	fmt.Printf("Created %d users successfully\n", len(users))

	// Now send friend requests
	fmt.Println("Sending friend requests...")
	for i, user := range users {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(userIndex int, u *User) {
			defer wg.Done()
			defer func() { <-semaphore }()

			for j := 0; j < requestsPerUser; j++ {
				// Pick a different user to send friend request to
				targetIndex := (userIndex + j + 1) % len(users)
				if targetIndex == userIndex {
					continue
				}
				target := users[targetIndex]

				err := client.SendFriendRequest(ctx, u.Token, target.ID)
				globalStats.IncrementTotal()
				
				if err != nil {
					globalStats.IncrementFailed()
				} else {
					globalStats.IncrementSuccess()
				}
			}
		}(i, user)
	}
	wg.Wait()

	globalStats.PrintStats()
	return nil
}

func runGroupChatTest(ctx context.Context, config *Config, groupCount int, membersPerGroup int, messagesPerMember int) error {
	fmt.Printf("\n========== Group Chat Test ==========\n")
	fmt.Printf("Creating %d groups with %d members each, sending %d messages per member...\n", 
		groupCount, membersPerGroup, messagesPerMember)
	globalStats.StartTime = time.Now()

	client := NewHTTPClient(config.HTTPEndpoint)

	// Create users
	totalUsers := groupCount * membersPerGroup
	users := make([]*User, 0, totalUsers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 50)

	fmt.Println("Creating users...")
	for i := 0; i < totalUsers; i++ {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			username := fmt.Sprintf("stress_group_%d_%d", time.Now().Unix(), index)
			password := "Test@123456"

			user, err := client.RegisterAndLogin(ctx, username, password)
			globalStats.IncrementTotal()
			
			if err != nil {
				globalStats.IncrementFailed()
			} else {
				globalStats.IncrementSuccess()
				mu.Lock()
				users = append(users, user)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	fmt.Printf("Created %d users successfully\n", len(users))

	// Create groups
	groups := make([]*Group, 0, groupCount)
	fmt.Println("Creating groups...")
	for i := 0; i < groupCount; i++ {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			startIdx := index * membersPerGroup
			if startIdx >= len(users) {
				return
			}

			creator := users[startIdx]
			groupName := fmt.Sprintf("StressGroup_%d", index)
			memberIDs := make([]int, 0, membersPerGroup-1)
			
			for j := 1; j < membersPerGroup && startIdx+j < len(users); j++ {
				memberIDs = append(memberIDs, users[startIdx+j].ID)
			}

			group, err := client.CreateGroup(ctx, creator.Token, groupName, memberIDs)
			globalStats.IncrementTotal()
			
			if err != nil {
				globalStats.IncrementFailed()
				log.Printf("Failed to create group %s: %v", groupName, err)
			} else {
				globalStats.IncrementSuccess()
				mu.Lock()
				groups = append(groups, group)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	fmt.Printf("Created %d groups successfully\n", len(groups))

	// Send messages to groups
	fmt.Println("Sending messages to groups...")
	for i, group := range groups {
		startIdx := i * membersPerGroup
		for j := 0; j < membersPerGroup && startIdx+j < len(users); j++ {
			user := users[startIdx+j]
			
			wg.Add(1)
			semaphore <- struct{}{}
			
			go func(u *User, g *Group) {
				defer wg.Done()
				defer func() { <-semaphore }()

				for k := 0; k < messagesPerMember; k++ {
					message := fmt.Sprintf("Test message %d from user %d", k, u.ID)
					err := client.SendTextMessage(ctx, u.Token, 2, g.ID, message)
					globalStats.IncrementTotal()
					globalStats.IncrementMessages()
					
					if err != nil {
						globalStats.IncrementFailed()
					} else {
						globalStats.IncrementSuccess()
					}
				}
			}(user, group)
		}
	}
	wg.Wait()

	globalStats.PrintStats()
	return nil
}

func runMixedOperationsTest(ctx context.Context, config *Config, duration int, concurrency int) error {
	fmt.Printf("\n========== Mixed Operations Test ==========\n")
	fmt.Printf("Running for %d seconds with %d concurrent users...\n", duration, concurrency)
	globalStats.StartTime = time.Now()

	client := NewHTTPClient(config.HTTPEndpoint)

	// Create initial users
	users := make([]*User, 0, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 50)

	fmt.Println("Creating initial users...")
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			username := fmt.Sprintf("stress_mixed_%d_%d", time.Now().Unix(), index)
			password := "Test@123456"

			user, err := client.RegisterAndLogin(ctx, username, password)
			globalStats.IncrementTotal()
			
			if err != nil {
				globalStats.IncrementFailed()
			} else {
				globalStats.IncrementSuccess()
				mu.Lock()
				users = append(users, user)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	fmt.Printf("Created %d users successfully\n", len(users))

	// Run mixed operations for the specified duration
	testCtx, cancel := context.WithTimeout(ctx, time.Duration(duration)*time.Second)
	defer cancel()

	fmt.Println("Running mixed operations...")
	for _, user := range users {
		wg.Add(1)
		
		go func(u *User) {
			defer wg.Done()

			for {
				select {
				case <-testCtx.Done():
					return
				default:
					// Randomly choose an operation
					op := time.Now().UnixNano() % 4
					
					switch op {
					case 0: // Send friend request
						if len(users) > 1 {
							targetIdx := int(time.Now().UnixNano()) % len(users)
							target := users[targetIdx]
							if target.ID != u.ID {
								err := client.SendFriendRequest(testCtx, u.Token, target.ID)
								globalStats.IncrementTotal()
								if err != nil {
									globalStats.IncrementFailed()
								} else {
									globalStats.IncrementSuccess()
								}
							}
						}
					case 1: // Send message
						if len(users) > 1 {
							targetIdx := int(time.Now().UnixNano()) % len(users)
							target := users[targetIdx]
							if target.ID != u.ID {
								message := fmt.Sprintf("Mixed test message at %s", time.Now().Format(time.RFC3339))
								err := client.SendTextMessage(testCtx, u.Token, 1, target.ID, message)
								globalStats.IncrementTotal()
								globalStats.IncrementMessages()
								if err != nil {
									globalStats.IncrementFailed()
								} else {
									globalStats.IncrementSuccess()
								}
							}
						}
					case 2: // Get friend list
						err := client.GetFriendList(testCtx, u.Token)
						globalStats.IncrementTotal()
						if err != nil {
							globalStats.IncrementFailed()
						} else {
							globalStats.IncrementSuccess()
						}
					case 3: // Get session list
						err := client.GetSessionList(testCtx, u.Token)
						globalStats.IncrementTotal()
						if err != nil {
							globalStats.IncrementFailed()
						} else {
							globalStats.IncrementSuccess()
						}
					}
					
					// Small delay to avoid overwhelming the server
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(user)
	}

	wg.Wait()
	globalStats.PrintStats()
	
	return nil
}

func runCleanup(configPath string) error {
	fmt.Println("\n========== Database Cleanup ==========")
	fmt.Println("WARNING: This will delete all test data!")
	fmt.Print("Are you sure you want to continue? (yes/no): ")
	
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	
	if response != "yes" {
		fmt.Println("Cleanup cancelled.")
		return nil
	}

	cleaner := NewDatabaseCleaner(configPath)
	if err := cleaner.Cleanup(); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	fmt.Println("Cleanup completed successfully!")
	return nil
}

// User represents a test user
type User struct {
	ID       int    `json:"user_id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

// Group represents a test group
type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MarshalJSON for User
func (u *User) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"user_id":  u.ID,
		"username": u.Username,
		"token":    u.Token,
	})
}
