package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// Format of a slack incoming webhook
type slackWebhookMessage struct {
	Text string `json:"text"`
}

// Simple webhook message
func Message(webhook *url.URL, text string, args ...interface{}) error {
	if webhook == nil || webhook.String() == "" {
		return nil
	}

	data := slackWebhookMessage{
		Text: fmt.Sprintf(text, args...),
	}
	json, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", webhook.String(), bytes.NewBuffer(json))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	_ = body // TODO check status
	return err
}
