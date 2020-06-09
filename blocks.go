package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/kelseyhightower/envconfig"
)

var config Config

// Config struct, filled with env variables from our Dockerfile
type Config struct {
	Interval      int    `default:"5"`
	WebhookURI    string `required:"true" split_words:"true"`
	SendgridToken string `required:"true" split_words:"true"`
	LastTimestamp int    `default:"-1" split_words:"true"`
}

// SlackWebhookBody represents the JSON body which we send to the Slack Webhook
type SlackWebhookBody struct {
	Text string `json:"text"`
}

// Block is an SendGrid block object as specified in:
// https://sendgrid.com/docs/API_Reference/Web_API_v3/blocks.html
type Block struct {
	Created int
	Email   string
	Reason  string
	Status  string
}

// Send a single block to the slack webhook
func sendMessage(block Block) {

	date := time.Unix(int64(block.Created), 0).String()
	message := fmt.Sprintf("Failed to send mail:\nCreated at: %s\nEmail: %s \nReason: %s\nStatus: %s\n", date, block.Email, block.Reason, block.Status)

	s := SlackWebhookBody{Text: message}
	b, _ := json.Marshal(s)
	_, err := http.Post(config.WebhookURI, "application/json", bytes.NewBuffer(b))

	if err != nil {
		fmt.Println(err)
	}

}

// Iterate through blocks, send your log messages and save new latest timestamp
func checkBlocks(blocks []Block) {
	for _, block := range blocks {
		sendMessage(block)
		if block.Created > config.LastTimestamp {
			config.LastTimestamp = block.Created + 1 // + 1 or we would get the last one all the time
		}
	}
}

// Makes the sendgrid api requests, parses the blocks and calls checkBlocks
func getBlocks() {
	URL := "https://api.sendgrid.com/v3/suppression/blocks?start_time=" + strconv.Itoa(config.LastTimestamp)

	req, err := http.NewRequest("GET", URL, nil)
	req.Header.Set("Authorization", "Bearer "+config.SendgridToken)

	if err != nil {
		fmt.Printf("Failed to build Request: %s\n", err)
		return
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("HTTP Call failed: %s\n", err.Error())
		return
	}
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	var blocks []Block
	err = decoder.Decode(&blocks)
	if err != nil {
		fmt.Printf("Failed parsing json: %T\n%s\n%#v\n", err, err, err)
		return
	}
	checkBlocks(blocks)
}

// Read environment variables (from docker) into config struct. Fails if variables are missing.
// Does not check for empty variables, that is your responsibility
func parseConfig() error {

	prefix := ""
	if err := envconfig.Process(prefix, &config); err != nil {
		log.Fatal(err.Error())
	}

	if config.LastTimestamp == -1 {
		config.LastTimestamp = int(time.Now().Unix())
	}
	return nil
}

func main() {

	err := parseConfig()

	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully created config")

	// Loop all da time!
	for range time.Tick(time.Duration(config.Interval) * time.Second) {
		getBlocks()
	}
}
