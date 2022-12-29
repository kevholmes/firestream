package main

import (
	"context"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func now() time.Time {
	return time.Now().UTC()
}

// setupCloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the OS. We then handle this by calling
// our clean up procedure and exiting the program.
func setupCloseHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-c:
			log.Debugf("SHUTDOWN: OS Interrupt received!")
			// cancel highest level context for firestream
			cancel()
			// finalize metrics and print out stats before exit
			//finalizeMetrics()
		case <-shutdownFirestreamImmediately:
			log.Debugf("shutdownFirestreamImmediately request recieved!")
			// cancel highest level context for firestream
			cancel()
			// finalize metrics and print out stats before exit
			//finalizeMetrics()
		}
	}
}

// help check for nil values and avoid panics w/ reflection
func IsNilInterface(i interface{}) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}

// Return a time.Time object from a provided UTC epoch time as a float64
func nanoEpochTimeObject(ts float64) (time.Time, error) {
	tsi := (int64(ts) * int64(time.Millisecond)) // Firebase takes seconds or nanoseconds not millis
	keyDate := time.Unix(0, tsi).UTC()
	return keyDate, nil
}

// Initialize global channels for use throughout Firestream
func initGlobalChannels() (ok bool) {
	// request a navajo update immediately
	navajoUpdater = make(chan bool)
	// request shutdown sequence initate
	shutdownFirestreamImmediately = make(chan bool)
	// init report processing channels for Stream API Reports -> Firestore
	firestoreAssembly = make(chan ReportDataStreamV1)
	transponderReportsV1 = make(chan TransponderReportDataStreamV1)
	videoReportsV1 = make(chan VideoReportDataStreamV1)
	eldReportsV1 = make(chan EldReportDataStreamV1)
	return ok
}
