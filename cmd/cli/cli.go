package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	DEFAULT_HOST = "localhost"
	DEFAULT_PORT = 6178
)

var (
	host string
	port int
	conn net.Conn
)

func Banner() {
	fmt.Println("=====================================")
	fmt.Println("      Welcome to In-Memory Store     ")
	fmt.Println("=====================================")
}

func Help() {
	fmt.Println("Available Commands:")
	fmt.Println("1. SET <key> <value>  - Insert or update a key-value pair")
	fmt.Println("2. GET <key>          - Retrieve the value for a given key")
	fmt.Println("3. DELETE <key>       - Delete a key-value pair")
	fmt.Println("4. EXIT               - Exit the application")
	fmt.Println("5. HELP               - Show this help message")
}

func InvalidCommand(cmd string) {
	fmt.Printf("Invalid command: %s\n", cmd)
	fmt.Println("Type 'HELP' to see the list of available commands.")
}

func ExitMessage() {
	fmt.Println("Exiting the application. Goodbye!")
}

func establishConnection() error {
	var err error
	fmt.Printf("Connecting to %s:%d... ", host, port)
	conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		fmt.Println("Failed!")
		return err
	}
	fmt.Println("Connected!")
	return nil
}

func closeConnection() {
	if conn != nil {
		conn.Close()
	}
}

func sendCommand(cmd string) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("not connected")
	}

	// Send command to the server
	_, err := fmt.Fprintf(conn, "%s\n", cmd)
	if err != nil {
		return "", err
	}

	// Read the response
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

func startInteractiveShell() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("kv> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle exit command locally
		if strings.ToUpper(input) == "EXIT" {
			ExitMessage()
			break
		}

		// Handle help command locally
		if strings.ToUpper(input) == "HELP" {
			Help()
			continue
		}

		// Send all other commands to the server
		response, err := sendCommand(input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			// Try to reconnect on error
			if err := establishConnection(); err != nil {
				fmt.Printf("Reconnection failed: %v\n", err)
			}
			continue
		}

		fmt.Println(response)
	}
}

var rootCmd = &cobra.Command{
	Use:   "kvstore",
	Short: "In-Memory Key-Value Store CLI",
	Long:  "In-memory key-value store interactive shell",
	Run: func(cmd *cobra.Command, args []string) {
		Banner()

		// Establish connection to the server
		if err := establishConnection(); err != nil {
			fmt.Printf("Failed to connect: %v\n", err)
			os.Exit(1)
		}
		defer closeConnection()

		fmt.Printf("Connected to KV Store server at %s:%d\n", host, port)
		Help()

		// Start interactive shell
		startInteractiveShell()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&host, "host", "H", DEFAULT_HOST, "Server hostname")
	rootCmd.Flags().IntVarP(&port, "port", "p", DEFAULT_PORT, "Server port")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
