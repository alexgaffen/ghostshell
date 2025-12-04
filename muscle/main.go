package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"golang.org/x/crypto/ssh"
)

// This matches the JSON the Python server expects
type BrainRequest struct {
	Command string `json:"command"`
}

type BrainResponse struct {
	Output string `json:"output"`
}

func main() {
	// 1. Configure the SSH Server
	config := &ssh.ServerConfig{
		// Allow ANY password (It's a trap!)
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			log.Printf("⚠️  Login attempt: User=%s Password=%s", c.User(), string(pass))
			return nil, nil // Return nil to allow login
		},
	}

	// 2. Load the Host Key we just generated
	privateBytes, err := ioutil.ReadFile("hostkey")
	if err != nil {
		log.Fatal("Failed to load host key: ", err)
	}
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}
	config.AddHostKey(private)

	// 3. Start Listening on Port 2222
	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		log.Fatal("Failed to listen on 2222: ", err)
	}
	log.Println("🪤  GhostShell Muscle (SSH) listening on port 2222...")

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept incoming connection:", err)
			continue
		}
		// Handle every connection in a separate Goroutine (Async!)
		go handleConnection(nConn, config)
	}
}

func handleConnection(nConn net.Conn, config *ssh.ServerConfig) {
	// Upgrade TCP connection to SSH
	_, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Println("Handshake failed:", err)
		return
	}

	// Discard global out-of-band requests
	go ssh.DiscardRequests(reqs)

	// Handle the channels (sessions)
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Println("Could not accept channel:", err)
			continue
		}

		// Handle PTY requests (so it feels like a real terminal)
		go func(in <-chan *ssh.Request) {
			for req := range in {
				req.Reply(req.Type == "shell" || req.Type == "pty-req", nil)
			}
		}(requests)

		// Start the Interactive Loop
		go runInteractiveShell(channel)
	}
}

func runInteractiveShell(channel ssh.Channel) {
	defer channel.Close()

	// Fake Welcome Message
	channel.Write([]byte("Welcome to Ubuntu 22.04.3 LTS (GNU/Linux 5.15.0-91-generic x86_64)\r\n"))
	channel.Write([]byte("System information as of " + "Wed Dec 3 20:00:00 UTC 2025" + "\r\n\r\n"))

	buffer := make([]byte, 1024)
	for {
		// 1. Show Prompt
		channel.Write([]byte("root@server:~# "))

		// 2. Read Input
		n, err := channel.Read(buffer)
		if err != nil {
			break
		}

		// Cleanup input (remove newlines/whitespace)
		// In a real terminal, we might need more complex parsing, but this works for basic commands
		cleanCmd := string(bytes.TrimSpace(buffer[:n]))
		
		if cleanCmd == "exit" {
			break
		}

		// 3. Send to Python Brain (Localhost:5000)
		// We define the JSON payload
		reqBody, _ := json.Marshal(BrainRequest{Command: cleanCmd})
		
		// Post to the Python service
		// NOTE: In Docker, this URL might change to "http://brain:5000", but for local testing "localhost" is fine.
		resp, err := http.Post("http://localhost:5000/hallucinate", "application/json", bytes.NewBuffer(reqBody))
		
		if err != nil {
			log.Println("Error contacting Brain:", err)
			channel.Write([]byte("\r\nSystem Error: AI Brain unreachable.\r\n"))
			continue
		}

		// 4. Read Response and Print to User
		var result BrainResponse
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		// Write the output (and add a carriage return/newline)
		channel.Write([]byte("\r\n" + result.Output + "\r\n"))
	}
}