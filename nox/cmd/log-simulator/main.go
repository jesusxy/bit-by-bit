package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

const (
	logFile               = "var/log/audit/audit.log"
	failedLoginTemplate   = "%s my-server ssh[%d]: Failed password for %s from %s port %d ssh2\n"
	acceptedLoginTemplate = "%s my-server sshd[%d]: Accepted password for %s from %s port %d ssh2\n"
)

func appendLog(message string) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}

	defer f.Close()

	if _, err := f.WriteString(message); err != nil {
		log.Printf("failed to write to log file: %v", err)
	}
}

func main() {
	log.Println("Starting log simulator....")

	for {
		sleepDuration := time.Duration(rand.Intn(4)+1) * time.Second
		time.Sleep(sleepDuration)

		timestamp := time.Now().Format("Jan  2 15:04:05")
		pid := rand.Intn(9000) + 1000
		port := rand.Intn(60000) + 1024

		scenario := rand.Intn(3)
		var logLine string

		switch scenario {
		case 0:
			// Scenario: A random failed login
			user := "admin"
			ip := fmt.Sprintf("10.10.10.%d", rand.Intn(254)+1)
			logLine = fmt.Sprintf(failedLoginTemplate, timestamp, pid, user, ip, port)
			fmt.Print("Injecting: Random failed login")
			appendLog(logLine)
		case 1:
			// Scenario: A successful login for test user
			user := "jsmith"
			ip := "8.8.8.8"
			logLine = fmt.Sprintf(acceptedLoginTemplate, timestamp, pid, user, ip, port)
			fmt.Print("Injecting: Successful login for jsmith")
			appendLog(logLine)
		}

		fmt.Println(" -> Done")
	}
}
