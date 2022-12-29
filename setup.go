package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

// some default config values for items that are not required from user
var DefaultmaxHugeDifferentialSetting float64 = 60000
var DefaultmaxJSONParseErrors float64 = 100
var DefaultNavajoIdMapRebuildTimer time.Duration = (120 * time.Second)
var DefaultWebsocketTimeout time.Duration = (20 * time.Second)

func parseEnvConfigs() error {
	// Environment variables in OS are config values
	// CLAPI (OAUTH 1.0a)
	const envClApiURL string = "CLAPI_HOST"
	const envClApiWSSURL string = "CLAPI_WSHOST"
	const envClApiOauthConsumerKey string = "CLAPI_KEY"
	const envClApiOauthConsumerSec string = "CLAPI_SEC"

	// maximum JSON parsing errors before readPump() and websocket are reset
	const envMaximumWebsocketParseErrors string = "MAX_JSON_ERRORS"

	// Navajo API (basic auth)
	const envNavajoHost string = "NAVAJO_URL"
	const envNavajoUser string = "NAVAJO_USER"
	const envNavajoPw string = "NAVAJO_PW"

	// Tunables
	const envMaxHugeDifferentialSetting string = "METRICS_HUGEDIFFIGNORE"
	const envMaxJsonParseErrors string = "JSON_ERRORS_BEFORE_RESTART"
	const envNavajoIdMapRebuildTimer string = "NAVAJO_MAP_REBUILD_TIMER" // ex "30s" for 30 second timer
	const envWebsocketTimeout string = "WEBSOCKET_TIMEOUT"               // ex "30s"

	// GCP - firestore, ...
	const envGoogleApplicationCredentials string = "GOOGLE_APPLICATION_CREDENTIALS"
	const envGoogleProjectId string = "GOOGLE_PROJECT_ID"

	// Default log level is INFO, turn on DEBUG level logging by setting DEBUG=true
	const envDebugLogLevel string = "DEBUG"

	// cl api oauth
	authConf.url = os.Getenv(envClApiURL)
	authConf.wsUrl = os.Getenv(envClApiWSSURL)
	authConf.OauthConsumerKey = os.Getenv(envClApiOauthConsumerKey)
	authConf.OauthConsumerSecret = os.Getenv(envClApiOauthConsumerSec)
	if authConf.url == "" || authConf.wsUrl == "" || authConf.OauthConsumerKey == "" || authConf.OauthConsumerSecret == "" {
		fmt.Printf("ERROR: Required environment vars are not set:\n")
		fmt.Printf("%s=%s\n", envClApiURL, authConf.url)
		fmt.Printf("%s=%s\n", envClApiWSSURL, authConf.wsUrl)
		fmt.Printf("%s=%s\n", envClApiOauthConsumerKey, authConf.OauthConsumerKey)
		fmt.Printf("%s=%s\n", envClApiOauthConsumerSec, authConf.OauthConsumerSecret)
		fmt.Printf("Please set a value for each environment variable listed.\n")
		return errors.New("EXIT FATAL: parsing environment variables")
	}
	// navajo auth
	navajoAuthConf.host = os.Getenv(envNavajoHost)
	// the user param isn't required with Navajo!
	navajoAuthConf.user = os.Getenv(envNavajoUser)
	navajoAuthConf.pass = os.Getenv(envNavajoPw)
	if navajoAuthConf.host == "" || navajoAuthConf.pass == "" {
		fmt.Printf("ERROR: Required environment vars are not set:\n")
		fmt.Printf("%s=%s\n", envNavajoHost, navajoAuthConf.host)
		fmt.Printf("%s=%s\n", envNavajoUser, navajoAuthConf.user)
		fmt.Printf("%s=%s\n", envNavajoPw, navajoAuthConf.pass)
		fmt.Printf("Please set a value for each environment variable listed.\n")
		return errors.New("EXIT FATAL: parsing environment variables")
	}
	// knobs that can be turned with defaults
	mxhdString, mxhdOk := os.LookupEnv(envMaxHugeDifferentialSetting)
	if !mxhdOk {
		// we'll take the default since it isn't set by the user..
		maxHugeDifferentialSetting = DefaultmaxHugeDifferentialSetting
	} else {
		var err error
		maxHugeDifferentialSetting, err = strconv.ParseFloat(mxhdString, 64)
		if err != nil {
			errMsg := fmt.Sprintf("EXIT FATAL: unable to set %s\n", envMaxHugeDifferentialSetting)
			return errors.New(errMsg)
		}
	}
	mxjsonerrorsString, mxjsonerrorsOk := os.LookupEnv(envMaxJsonParseErrors)
	if !mxjsonerrorsOk {
		// take the default
		maxJSONParseErrors = DefaultmaxJSONParseErrors
	} else {
		var err error
		maxJSONParseErrors, err = strconv.ParseFloat(mxjsonerrorsString, 64)
		if err != nil {
			errMsg := fmt.Sprintf("EXIT FATAL: unable to set %s\n", envMaxJsonParseErrors)
			return errors.New(errMsg)
		}
	}
	njRebuildTimer, njRebuildTimeOk := os.LookupEnv(envNavajoIdMapRebuildTimer)
	if !njRebuildTimeOk {
		// take the default
		navajoRebuildTimer = DefaultNavajoIdMapRebuildTimer
		log.Infof("Using default %s setting of: %v\n", envNavajoIdMapRebuildTimer, navajoRebuildTimer)
	} else {
		var err error
		navajoRebuildTimer, err = time.ParseDuration(njRebuildTimer)
		log.Infof("Using custom %s setting of: %v\n", envNavajoIdMapRebuildTimer, navajoRebuildTimer)
		if err != nil {
			errMsg := fmt.Sprintf("EXIT FATAL: unable to set %s\n", envNavajoIdMapRebuildTimer)
			return errors.New(errMsg)
		}
	}
	wsTimeout, wsTimeoutOk := os.LookupEnv(envWebsocketTimeout)
	if !wsTimeoutOk {
		// take the default
		websocketTimeout = DefaultWebsocketTimeout
		log.Infof("Using default %s setting of: %s\n", envWebsocketTimeout, websocketTimeout)
	} else {
		var err error
		websocketTimeout, err = time.ParseDuration(wsTimeout)
		log.Infof("Using custom %s setting of: %s\n", envWebsocketTimeout, websocketTimeout)
		if err != nil {
			errMsg := fmt.Sprintf("EXIT FATAL: unable to set %s\n", envWebsocketTimeout)
			return errors.New(errMsg)
		}
	}
	// GCP - the gcp libraries will auto-config your GCP API access when
	// run within GCP's cloud environment. This app isn't always somewhere
	// where auto-detect works, so we enforce that this service key is set to something..
	_, gcpAppCredsOk := os.LookupEnv(envGoogleApplicationCredentials)
	if !gcpAppCredsOk {
		// yell at user and return error so we can quit
		errMsg := fmt.Sprintf("ERROR: Required GCP service account file path not found in env var: %s\n", envGoogleApplicationCredentials)
		return errors.New(errMsg)
	}
	// project is required for firestore client init
	gcpProjectId = os.Getenv(envGoogleProjectId)
	if gcpProjectId == "" {
		errMsg := fmt.Sprintf("ERROR: Required GCP Project ID not found in env var: %s\n", envGoogleProjectId)
		return errors.New(errMsg)
	}

	// Logging level
	enableDebug, enableDebugOk := os.LookupEnv(envDebugLogLevel)
	if !enableDebugOk {
		// set our default level
		log.SetLevel(log.InfoLevel)
	} else if enableDebug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Debugln("Set logging level to Debug")
	}

	return nil
}
