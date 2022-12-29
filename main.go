package main

import (
	"context"
	"os"
	"time"

	joonix "github.com/joonix/log"
	log "github.com/sirupsen/logrus"
)

// Global Variables

// App version / build info
// go build -ldflags "-X main.appBuildTime=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.appGitHash=`git rev-parse HEAD`"
var appGitHash, appBuildTime string = "", ""

// global vars for metrics usage, mutex required for FirebaseMetrics
var imetrics IngestedMetrics
var fmmetrics FirebaseMetrics
var maxHugeDifferentialSetting float64

// clapi and navajo auth config objects
var authConf CLAPIOauthConfig
var navajoAuthConf NavajoAuthConfig

// global var including CL API Ids -> Cartwheel Ids mapping
var navajoReferenceIds NavajoAccountData

// global channels
var navajoUpdater chan bool
var shutdownFirestreamImmediately chan bool
var firestoreAssembly chan ReportDataStreamV1
var transponderReportsV1 chan TransponderReportDataStreamV1
var videoReportsV1 chan VideoReportDataStreamV1
var eldReportsV1 chan EldReportDataStreamV1

// global tuner knobs
var maxJSONParseErrors float64
var navajoRebuildTimer time.Duration // triggers navajo id mapping
var websocketTimeout time.Duration   // triggers websocket reset if no data within duration

// GCP project config
var gcpProjectId string

func init() {
	// Disable log prefixes such as the default timestamp.
	// Prefix text prevents the message from being parsed as JSON.
	// A timestamp is added when shipping logs to Cloud Logging.
	//log.SetFlags(0) // default golang log pkg - turn off timestamps
	//log.SetFormatter(&log.JSONFormatter{}) // default logrus json formatter
	log.SetFormatter(joonix.NewFormatter()) // default stackdriver log formatter
	// initialize our global channels
	initGlobalChannels()
}

func main() {
	// get configs from environment
	err := parseEnvConfigs()
	if err != nil {
		log.Errorln(err)
		os.Exit(1)
	}

	// Base context object is empty, we'll immediately create a copy
	// of our parent context with a Done channel. This new Done channel
	// is closed when the cancel() func returned is called, and it will
	// also make all copies of this context with their own Done channel
	// return immediately.
	ctx, cancel := context.WithCancel(context.Background())

	// catch sig term and ctrl+c etc, tell our goroutines to return that are spun off ctx
	go setupCloseHandler(cancel)

	// init our firestore pipeline workers

	// init global metrics objects ..
	imetrics.transpondersWithNoAccountId = make(map[float64]bool)

	// init global objects who are periodically updated to contain CL API Ids -> Cartwheel Ids
	navajoReferenceIds.clAccountIdMap = make(map[string]string)
	navajoReferenceIds.clDeviceIdMap = make(map[string]string)

	// init go routine to perodically poll navajo and build account/transponder id maps
	go keepNavajoIdMapsUpdated(ctx)
	navajoUpdater <- true // request navajo map rebuild immediately

	// make sure our ref maps are available before opening stream..
	// we can fail on navajo updates occassionally later without issues
	// but failing at boot means we have no device or account ids to match
	// incoming streaming data with
	for len(navajoReferenceIds.clAccountIdMap) == 0 || len(navajoReferenceIds.clDeviceIdMap) == 0 {
		if navajoReferenceIds.failedUpdates > 0 {
			log.Errorln("FATAL: Cannot init connection to Navajo API at startup! Bailing out!")
			shutdownFirestreamImmediately <- true // tell app to log basic metrics before cancelling main context
			break
		}
		log.Debug("Waiting on Navajo ID map population..")
		time.Sleep(2 * time.Second)
	}

	// launch assembly pipeline router
	go firestoreAssemblyRouter(ctx)
	// launch event report_data assembler workers
	for i := 0; i < 5; i++ {
		c, err := createFirestoreClient(ctx)
		if err != nil {
			log.Errorln("ERROR FATAL: Unable to create firestore Report Data clients at Firestream init!")
			shutdownFirestreamImmediately <- true
		}
		go transponderReportWriterV1(ctx, c)
	}
	/*
		// launch video data assemblers
		for i := 0; i < 2; i++ {
			c, err := createFirestoreClient(ctx)
			if err != nil {
				log.Errorln("ERROR FATAL: Unable to create firestore Video Data clients at Firestream init!")
				shutdownFirestreamImmediately <- true
			}
			go videoReportWriterV1(ctx, c)
		}*/
	// launch eld data assemblers
	for i := 0; i < 2; i++ {
		c, err := createFirestoreClient(ctx)
		if err != nil {
			log.Errorln("ERROR FATAL: Unable to create firestore Video Data clients at Firestream init!")
			shutdownFirestreamImmediately <- true
		}
		go eldReportWriterV1(ctx, c)
	}

	// launch websocket ingestion goroutine
	go websocketIngestor(ctx)

	log.Infof("Firestream %s:%s is running...", appBuildTime, appGitHash)
	for {
		select {
		case <-ctx.Done():
			log.Debugln("main(): context.Done() received")
			time.Sleep(200 * time.Millisecond) // allow any final metrics tasks to finish
			log.Exit(0)
		}
	}
}
