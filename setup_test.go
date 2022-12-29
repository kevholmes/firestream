package main

import (
	"os"
	"testing"
)

// Set required environment vars and then parse them all in via parseEnvConfigs()
func TestParseEnvConfigs(t *testing.T) {
	envVars := map[string]string{
		"CLAPI_HOST":                     "http://127.0.0.1/v2/open_stream/stream_name",
		"CLAPI_WSHOST":                   "ws://127.0.0.1/v2/open_stream/stream_name",
		"CLAPI_KEY":                      "oauthKey",
		"CLAPI_SEC":                      "oauthSecret123",
		"NAVAJO_URL":                     "http://127.0.0.1:8029",
		"NAVAJO_USER":                    "",
		"NAVAJO_PW":                      "test",
		"GOOGLE_APPLICATION_CREDENTIALS": "firestream.json",
		"GOOGLE_PROJECT_ID":              "test-project",
	}
	// set our keys
	for key, value := range envVars {
		err := os.Setenv(key, value)
		if err != nil {
			t.Errorf("TestParseEnvConfigs(): Error setting environment variables prior to test")
		}
	}
	err := parseEnvConfigs()
	if err != nil {
		t.Errorf("parseEnvConfigs(%s) = error", envVars)
	}

}
