package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

// Global config variables
var (
	amiHost     string
	amiUser     string
	amiPassword string
	backendURL  string
)

func loadConfig() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		fmt.Println("Error loading config.ini:", err)
		return
	}

	amiHost = cfg.Section("AMI").Key("host").String()
	amiUser = cfg.Section("AMI").Key("user").String()
	amiPassword = cfg.Section("AMI").Key("password").String()
	backendURL = cfg.Section("Backend").Key("url").String()
}

func main() {
	loadConfig()

	conn, err := net.Dial("tcp", amiHost)
	if err != nil {
		fmt.Println("Error connecting to AMI:", err)
		return
	}
	defer conn.Close()

	loginCmd := fmt.Sprintf("Action: Login\r\nUsername: %s\r\nSecret: %s\r\n\r\n", amiUser, amiPassword)
	_, err = conn.Write([]byte(loginCmd))
	if err != nil {
		fmt.Println("Error sending login request:", err)
		return
	}

	fmt.Println("Connected to Asterisk AMI, listening for CustomCallEvent logs...")
	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading data:", err)
			break
		}

		logLine := strings.TrimSpace(line)

		if strings.HasPrefix(logLine, "AppData: CustomCallEvent") {
			jsonOutput := parseLogToJSON(logLine)
			postToBackend(jsonOutput) // JSONni backendga jo'natish
		}
	}
}

func parseLogToJSON(log string) string {
	data := extractFields(log)
	event := CallEvent{
		Status: data["Status"],
	}
	event.Data.Caller = parseInt(data["Caller"])
	event.Data.Dest = parseInt(data["Dest"])
	event.Data.StartTime = data["StartTime"]
	event.Data.EndTime = defaultString(data["EndTime"], "0")
	event.Data.AnswerTime = defaultString(data["AnswerTime"], "0")
	event.Data.Duration = parseInt(data["Duration"])
	event.Data.BillableSeconds = parseInt(data["BillableSeconds"])

	jsonData, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return `{"error": "Failed to generate JSON"}`
	}
	return string(jsonData)
}

func postToBackend(jsonStr string) {
	resp, err := http.Post(backendURL, "application/json", bytes.NewBuffer([]byte(jsonStr)))
	if err != nil {
		fmt.Println("Error posting to backend:", err)
		return
	}
	defer resp.Body.Close()
}

func extractFields(log string) map[string]string {
	fields := make(map[string]string)
	regex := regexp.MustCompile(`(\w+): ([^,]+)`)
	matches := regex.FindAllStringSubmatch(log, -1)

	for _, match := range matches {
		if len(match) == 3 {
			fields[match[1]] = strings.TrimSpace(match[2])
		}
	}

	return fields
}

func defaultString(value, defaultVal string) string {
	if value == "" {
		return defaultVal
	}
	return value
}

func parseInt(value string) int {
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return num
}

type CallEvent struct {
	Status string `json:"status"`
	Data   struct {
		Caller          int    `json:"Caller"`
		Dest            int    `json:"Dest"`
		StartTime       string `json:"startTime"`
		EndTime         string `json:"endTime"`
		AnswerTime      string `json:"answerTime"`
		Duration        int    `json:"duration"`
		BillableSeconds int    `json:"billAbleSeconds"`
	} `json:"data"`
}
