package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func authenticationError(body []byte) error {
	var response struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &response) == nil {
		if message := strings.TrimSpace(response.Error); message != "" {
			return fmt.Errorf("authentication error: %s", message)
		}
	}
	return errors.New("authentication error: API key invalid or inactive")
}
