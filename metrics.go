package main

import (
	"sync"

	gabs "github.com/Jeffail/gabs/v2"
	log "github.com/sirupsen/logrus"
)

// global vars for metrics
type IngestedMetrics struct {
	// mu sync.Mutex // reader/metrics do-er is single threaded for now - no need for a mutex
	minDifferential             float64          // smallest differential found between reportTimestamp and now()
	maxDifferential             float64          // largest differential found between reportTimestamp and now()
	dumpDifferential            float64          // valid differential measurements dump
	totalSamples                float64          // total valid samples. dumpDifferential / totalSamples = avg diff
	unparseableSamples          float64          // samples that couldn't be parsed via gabs.ParseJSON()
	maxHugeDifferential         float64          // samples larger than maxHugeDifferentialSetting, not included in average diff
	totalReadPumpRestarts       float64          // counter for times readPump() has had to restart, usually due to web socketconnection reset
	gsmTransponderReports       float64          // counter for total number of GSM transponder generated reports ingested
	lteTransponderReports       float64          // counter for total number of LTE transponder generated reports ingested
	reportsWithNoAccountId      float64          // counter for total number of reports ingested that didn't have an accountId
	transpondersWithNoAccountId map[float64]bool // track what transponders are sending data without a CL API accountId field
	reportsWithNoDataType       float64          // track incoming stream reports that do not include the "dataType" field
	reportsWithNoType           float64          // track incoming stream reports that do not include the "type" field
}

type FirebaseMetrics struct {
	mu                sync.Mutex // Firestore writing threads are go routines
	writers           float64    // active firestore write goroutines
	written           float64    // tally of total records written to firestore
	statusWrites      float64    // tally of total status records written to firestore
	tripWrites        float64    // tally of total trip records written to firestore
	overspeedWrites   float64    // tally of total overspeeding records written to firestore
	hardAccelWrites   float64    // tally of total hard_accel records written to firestore
	hardBrakingWrites float64    // tally of total hard_braking records written to firestore
	videoEventWrites  float64    // tally of total video event records written to firestore
	stopWrites        float64    // tally of total stop records written to firestore
}

func finalizeMetrics() {
	log.Infof("max latency differential: %f\n", imetrics.maxDifferential)
	log.Infof("min latency differential: %f\n", imetrics.minDifferential)
	avgDiff := imetrics.dumpDifferential / imetrics.totalSamples
	log.Infof("average latency differential: %f\n", avgDiff)
	log.Infof("valid samples (not including reports with diff greater-than %v seconds ): %v\n", maxHugeDifferentialSetting, imetrics.totalSamples)
	log.Infof("unparseable samples: %v\n", imetrics.unparseableSamples)
	log.Infof("LTE transponder reports with differentials over %v seconds (not included in avgs or max diff values): %v\n", maxHugeDifferentialSetting, imetrics.maxHugeDifferential)
	log.Infof("total LTE reports ingested: %v\n", imetrics.lteTransponderReports)
	log.Infof("total GSM reports ingested: %v (ignored when generating metrics)\n", imetrics.gsmTransponderReports)
	for k, v := range imetrics.transpondersWithNoAccountId {
		log.Warnf("transponder: %v -- no accountId? %v\n", k, v)
	}
	log.Infof("total reports ingested that didn't include dataType field: %v\n", imetrics.reportsWithNoDataType)
	log.Infof("total reports ingested that didn't include type field: %v\n", imetrics.reportsWithNoType)
}

// This thing is pretty lame
func collectIngestionMetrics(jsonParsed *gabs.Container) {
	transponderId, ok := jsonParsed.Path("transponderId").Data().(float64)
	if !ok {
		log.Debugf("Metrics can't get transponderId ... skipping accountId detection\n")
	} else {
		_, ok := jsonParsed.Path("accountId").Data().(int)
		if !ok {
			log.Debugf("Metrics can't get accountId for transponder %v\n", transponderId)
			imetrics.reportsWithNoAccountId++
			imetrics.transpondersWithNoAccountId[transponderId] = true
		} else {
			imetrics.transpondersWithNoAccountId[transponderId] = false
		}
	}
	_, ok = jsonParsed.Path("dataType").Data().(string)
	if !ok {
		imetrics.reportsWithNoDataType++
	}
	_, ok = jsonParsed.Path("type").Data().(string)
	if !ok {
		imetrics.reportsWithNoType++
	}
	reportTimestamp, ok := jsonParsed.Path("data.reportTimestamp").Data().(float64)
	if !ok {
		log.Debugf("Can't get reportTimestamp ... skipping latency metrics collection!\n")
	} else {
		// lets get some latency data to see if this will work for a "live map"
		now := float64(makeTimestamp())
		differential := now - reportTimestamp
		if transponderId < 519000 /*&& reportType == "status"*/ { // looking for GSM transponders
			imetrics.gsmTransponderReports++
		} else if transponderId > 519000 /*&& reportType == "status"*/ { // looking for LTE transponders sending status reports
			if differential <= imetrics.minDifferential {
				imetrics.minDifferential = differential
				imetrics.totalSamples++ // track how many timestamps we're looking at
				imetrics.dumpDifferential += differential
			} else if differential <= maxHugeDifferentialSetting {
				if differential > imetrics.maxDifferential {
					imetrics.maxDifferential = differential
				}
				imetrics.totalSamples++ // track how many timestamps we're looking at
				imetrics.dumpDifferential += differential
			} else if differential > maxHugeDifferentialSetting {
				imetrics.maxHugeDifferential++
			}
			// log.Debugf("latency diff: %v : %.0f\n", transponderId, differential)
			imetrics.lteTransponderReports++

		}
	}
}
