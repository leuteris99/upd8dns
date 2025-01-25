package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

const (
	ipServiceURL = "https://api.ipify.org" // URL of service to get public IP
)

func getPublicIP() (string, error) {
	resp, err := http.Get(ipServiceURL)
	if err != nil {
		return "", fmt.Errorf("failed to get public IP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get public IP: HTTP status %d", resp.StatusCode)
	}

	ipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IP from response: %w", err)
	}

	ip := strings.TrimSpace(string(ipBytes))
	return ip, nil
}

func updateCloudflareRecord(api *cloudflare.API, zoneID string, recordName string, recordType string, newIP string, currentIP string) {
	ctx := context.Background()

	if newIP == currentIP {
		fmt.Println("IP address has not changed. Skipping update")
		return
	}

	records, _, err := api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
	if err != nil {
		log.Fatalf("Error listing DNS records: %v", err)
	}

	recordID := ""
	for _, record := range records {
		if record.Type == recordType && record.Name == recordName {
			recordID = record.ID
			break
		}
	}

	if recordID == "" {
		// 6a. If not found, create the record
		fmt.Printf("DNS Record %s does not exist, creating...\n", recordName)

		nr := cloudflare.CreateDNSRecordParams{
			Type:    recordType,
			Name:    recordName,
			Content: newIP,
		}

		_, err = api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), nr)
		if err != nil {
			log.Fatalf("Error creating DNS record: %v", err)
		}
		fmt.Println("DNS record created successfully!")
	} else {
		// 6b. If found, update the record
		fmt.Printf("DNS Record %s found, updating...\nFrom %s to %s\n", recordName, currentIP, newIP)
		updatedRecord := cloudflare.UpdateDNSRecordParams{
			Type:    recordType,
			Name:    recordName,
			Content: newIP,
			ID:      recordID,
		}

		_, err = api.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), updatedRecord)
		if err != nil {
			log.Fatalf("Error updating DNS record: %v", err)
		}
		fmt.Println("DNS record updated successfully!")
	}
}

func runService() {

	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	zoneID := os.Getenv("CLOUDFLARE_ZONE_ID")
	recordName := os.Getenv("CLOUDFLARE_DNS_RECORD_NAME")
	recordType := os.Getenv("CLOUDFLARE_DNS_RECORD_TYPE")
	updateInterval, err := strconv.Atoi(os.Getenv("INTERVAL")) // How often to check for IP changes
	if err != nil {
		log.Fatal("Error: Can't read the interval from the environment.")
	}

	ticker := time.NewTicker(time.Duration(updateInterval) * time.Minute)

	if apiToken == "" || zoneID == "" || recordName == "" || recordType == "" {
		log.Fatal("Error: CLOUDFLARE_API_TOKEN, CLOUDFLARE_ZONE_ID, CLOUDFLARE_DNS_RECORD_NAME, and CLOUDFLARE_DNS_RECORD_TYPE environment variables must be set.")
	}

	api, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		log.Fatalf("Error initializing Cloudflare API client: %v", err)
	}

	var currentIP string

	for {

		newIP, err := getPublicIP()
		if err != nil {
			log.Printf("Error getting public IP: %v", err)
			<-ticker.C
			continue
		}

		updateCloudflareRecord(api, zoneID, recordName, recordType, newIP, currentIP)
		currentIP = newIP

		// time.Sleep(updateInterval)
		<-ticker.C
	}
}

func main() {
	runService()
}
