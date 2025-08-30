package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

const (
	logFile               = "testdata/auth.log"
	failedLoginTemplate   = "%s my-server sshd[%d]: Failed password for %s from %s port %d ssh2\n"
	acceptedLoginTemplate = "%s my-server sshd[%d]: Accepted password for %s from %s port %d ssh2\n"
	execsnoopTemplate     = "%s %-16s %-7d %-7d %-3d %s\n"
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
		sleepDuration := time.Duration(rand.Intn(3)+1) * time.Second
		time.Sleep(sleepDuration)

		scenario := rand.Intn(3)
		var logLine, logMessage string

		switch scenario {
		case 0:
			// Scenario: A random failed login
			logMessage = "Injecting: Random failed login"
			timestamp := time.Now().Format("Jan  2 15:04:05")
			pid := rand.Intn(9000) + 1000
			port := rand.Intn(60000) + 1024
			user := "admin"
			ip := fmt.Sprintf("10.10.10.%d", rand.Intn(254)+1)
			logLine = fmt.Sprintf(failedLoginTemplate, timestamp, pid, user, ip, port)
		case 1:
			// Scenario: A successful login for test user
			logMessage = "Injecting: Successful login for jsmith"
			pid := rand.Intn(9000) + 1000
			port := rand.Intn(60000) + 1024
			user := "jsmith"
			timestamp := time.Now().Format("Jan  2 15:04:05")
			ip := "8.8.8.8"
			logLine = fmt.Sprintf(acceptedLoginTemplate, timestamp, pid, user, ip, port)
		case 2:
			logMessage = "Injecting: suspicious execsnoop event"
			timestamp := time.Now().Format("15:04:05")
			pid := rand.Intn(90000) + 1000
			ppid := rand.Intn(90000) + 1000
			command := "nmap"
			fullCommand := "/usr/bin/nmap -p 1-65535 localhost"
			logLine = fmt.Sprintf(execsnoopTemplate, timestamp, command, pid, ppid, 0, fullCommand)
		}

		if logLine != "" {
			fmt.Print(logMessage)
			appendLog(logLine)
			fmt.Println(" -> Done")
		}
	}
}
