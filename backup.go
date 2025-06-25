package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runCommand(cmd string, args ...string) string {
	command := exec.Command(cmd, args...)
	var out bytes.Buffer
	command.Stdout = &out
	err := command.Run()
	if err != nil {
		log.Fatalf("Error running command: %v", err)
	}
	return out.String()
}

func createImageBackup(linodeID string) {
	// Step 1: Delete old images
	fmt.Println("Deleting old images for Linode ID", linodeID)
	images := runCommand("linode-cli", "images", "list", "--json")
	// Use jq or a similar parser to filter the output for specific linodeID.
	// Assuming that you have jq installed:
	imageIDs := runCommand("jq", "-r", fmt.Sprintf(`.[] | select(.description | contains("Backup of Linode ID %s")) | .id`, linodeID))
	for _, imageID := range strings.Split(imageIDs, "\n") {
		if imageID != "" {
			fmt.Println("Deleting image ID:", imageID)
			runCommand("linode-cli", "images", "delete", imageID)
		}
	}

	// Step 2: Get the disk ID of the Linode
	fmt.Println("Getting disk ID for Linode ID", linodeID)
	diskID := runCommand("linode-cli", "linodes", "disks-list", linodeID, "--json")
	diskID = runCommand("jq", "-r", ".[0].id", diskID)
	if diskID == "" {
		log.Fatalf("No disk found for Linode ID %s. Backup cannot proceed.", linodeID)
	}

	// Step 3: Create a new image
	imageLabel := fmt.Sprintf("Backup-%s", time.Now().Format("20060102-1504"))
	fmt.Println("Creating new image with label:", imageLabel)
	runCommand("linode-cli", "images", "create", "--disk_id", diskID, "--label", imageLabel, "--description", fmt.Sprintf("Backup of Linode ID %s", linodeID))
	fmt.Println("Backup created successfully with label:", imageLabel)
}

func scheduleCronJobs(linodeID, startTime, endTime string) {
	// Convert start and end times to cron format
	startHour := strings.Split(startTime, ":")[0]
	startMinute := strings.Split(startTime, ":")[1]
	endHour := strings.Split(endTime, ":")[0]
	endMinute := strings.Split(endTime, ":")[1]

	// Adjust end time to take a backup 5 minutes before the scheduled shutdown
	endMinuteInt := stringToInt(endMinute)
	if endMinuteInt < 5 {
		endMinute = fmt.Sprintf("%02d", endMinuteInt+55)
		endHour = fmt.Sprintf("%02d", stringToInt(endHour)-1)
	} else {
		endMinute = fmt.Sprintf("%02d", endMinuteInt-5)
	}

	// Create a cron job to start the Linode
	startCron := fmt.Sprintf("%s %s * * * /usr/bin/linode-cli linodes boot %s", startMinute, startHour, linodeID)
	runCommand("bash", "-c", fmt.Sprintf("(crontab -l 2>/dev/null; echo \"%s\") | crontab -", startCron))

	// Create a cron job to shut down the Linode and take a backup before shutting it down
	endCron := fmt.Sprintf("%s %s * * * /root/manage_linode.sh %s", endMinute, endHour, linodeID)
	runCommand("bash", "-c", fmt.Sprintf("(crontab -l 2>/dev/null; echo \"%s\") | crontab -", endCron))

	// Cron job to shut down the Linode
	shutdownCron := fmt.Sprintf("%s %s * * * /usr/bin/linode-cli linodes shutdown %s", strings.Split(endTime, ":")[1], strings.Split(endTime, ":")[0], linodeID)
	runCommand("bash", "-c", fmt.Sprintf("(crontab -l 2>/dev/null; echo \"%s\") | crontab -", shutdownCron))

	fmt.Println("Cron jobs set up successfully!")
}

func stringToInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func main() {
	var linodeID, startTime, endTime string

	// Get Linode ID
	fmt.Print("Enter the Linode ID you want to work on: ")
	fmt.Scan(&linodeID)

	// Check if Linode is running
	status := runCommand("linode-cli", "linodes", "view", linodeID, "--json")
	if !strings.Contains(status, "\"status\": \"running\"") {
		fmt.Println("The Linode is not running. Starting it...")
		runCommand("linode-cli", "linodes", "boot", linodeID)
		time.Sleep(5 * time.Second) // Wait for Linode to start
	}

	// Get working hours
	fmt.Print("Enter working start time (HH:MM, 24-hour format): ")
	fmt.Scan(&startTime)
	fmt.Print("Enter working end time (HH:MM, 24-hour format): ")
	fmt.Scan(&endTime)

	// Set up cron jobs
	scheduleCronJobs(linodeID, startTime, endTime)

	// Create image backup before the shutdown
	createImageBackup(linodeID)
}
