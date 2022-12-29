package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"cloud.google.com/go/firestore"
	"google.golang.org/genproto/googleapis/type/latlng"
)

// Handle inbound structs from upstream report router
func transponderReportWriterV1(ctx context.Context, c *firestore.Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case rds := <-transponderReportsV1:
			log.Debugf("transponderReportWriterV1() received new report: %s:%s to process into Firestore...\n", rds.reportType, rds.reportDataType)

			// validate and populate our report struct
			ok := rds.build()
			if !ok {
				log.Warnf("Unable to validate and build a TransponderReportDataStreamV1 object sent from upstream channel transponderReportsV1\n")
				break // unable to validate the packet, drop it and move on
			}

			// build firestore reference
			ref := rds.firestoreReference(c)

			// marshall our report data streaming record into a firestore status record
			record, err := rds.firestoreRecord()
			if err != nil {
				log.Errorf("Unable to marshall streaming JSON report to Firestore record: %s", rds.json.String())
				break // don't try to write incomplete packet to Firestore
			}

			// check result
			result, err := ref.NewDoc().Set(ctx, record)
			if err != nil {
				log.Errorf("Firestore write error: %v", err)
			} else {
				log.Debugf("Firestore write result: %v", result)
			}
			// go back to waiting for a new report to enter channel
		}
	}
}

// Methods on *TransponderReportDataStreamV1
// These methods are intentionally left unversioned for now.

// create a firestore reference location on a V1 transponder report
func (r *TransponderReportDataStreamV1) firestoreReference(c *firestore.Client) *firestore.CollectionRef {
	ref := c.Collection("account").Doc(r.cwAccountId).Collection("vehicle").Doc(r.cwDeviceWebId).Collection("report_data")
	return ref
}

// validate/populate all items needed for an actionable TransponderReportDataStreamV1 type
func (r *TransponderReportDataStreamV1) build() (ok bool) {
	// transponder reports require a transponderId and accountId
	transponderIdf, tIdOk := r.json.Path("transponderId").Data().(float64)
	clApiAcctIdf, aIdOk := r.json.Path("accountId").Data().(float64)
	if !tIdOk || !aIdOk {
		log.Warnf("TransponderReportDataStreamV1.build(): report doesn't contain required value(s): transponderId:%t, accountId:%t\n", tIdOk, aIdOk)
		return false
	}
	transponderId := fmt.Sprintf("%.0f", transponderIdf)
	clApiAcctId := fmt.Sprintf("%.0f", clApiAcctIdf)
	// get our clapi ids <-> cartwheel ids out of global map (mutex ahoy)
	ok = r.cartwheelMap(transponderId, clApiAcctId)
	if !ok {
		log.Warnf("TransponderReportDataStreamV1.build(): Unable to obtain cartwheel accountId or web id from in-memory mapping (transponderId): %v\n", transponderId)
		return false
	}
	return true
}

// fetch / insert cartwheel ids from in-memory map into our record
func (r *TransponderReportDataStreamV1) cartwheelMap(t string, a string) (ok bool) {
	navajoReferenceIds.mutex.Lock()
	defer navajoReferenceIds.mutex.Unlock()
	// check global struct NavajoReferenceIds for matches
	cwAccountId, acctOk := navajoReferenceIds.clAccountIdMap[a]
	if !acctOk {
		log.Warnf("getCartwheelIds() unable to find match for CL API AccountId (%v) : Navajo Account Id\n", a)
		return false
	}
	cwDeviceId, deviceOk := navajoReferenceIds.clDeviceIdMap[t]
	if !deviceOk {
		log.Warnf("getCartwheelIds() unable to find match for CL API TransponderId (%v) : Navajo Device Id\n", t)
		return false
	}
	r.cwAccountId = cwAccountId
	r.cwDeviceWebId = cwDeviceId
	return true
}

// parse a streaming report JSON packet for configId
func (r *TransponderReportDataStreamV1) reportConfigId() (float64, bool) {
	configId, ok := r.json.Path("data.configId").Data().(float64)
	if ok {
		return configId, true
	} else {
		return 0, false
	}
}

// parse a streaming report JSON packet for duration key
func (r *TransponderReportDataStreamV1) reportDuration() (float64, bool) {
	duration, ok := r.json.Path("data.duration").Data().(float64)
	if ok {
		return duration, true
	} else {
		return 0, false
	}
}

// parse a streaming report JSON packet for eventStart key
func (r *TransponderReportDataStreamV1) reportEventStart() (time.Time, bool) {
	// convert float64 as milliseconds since epoch to time.Time
	eventStart, ok := r.json.Path("data.eventStart").Data().(float64)
	if ok {
		// there's a float64 there as UTC epoch time. convert to time.Time
		es, _ := nanoEpochTimeObject(eventStart)
		return es, true
	} else {
		return time.Time{}, false
	}
}

// parse a streaming report JSON packet for inProgress
func (r *TransponderReportDataStreamV1) reportInProgress() (rip bool, ok bool) {
	ip, ok := r.json.Path("data.inProgress").Data().(bool)
	if ok {
		return ip, true
	} else {
		return false, false
	}
}

// parse streaming report JSON packet for location accuracy key
func (r *TransponderReportDataStreamV1) reportLocationAccuracy() (rla float64, ok bool) {
	locationAccuracy, ok := r.json.Path("data.location.accuracy").Data().(float64)
	if ok {
		rla = locationAccuracy
		return rla, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet location coordinates lat, long
// returns a "latlng.LatLng" pointer
func (r *TransponderReportDataStreamV1) reportGeoObj() (g *latlng.LatLng, ok bool) {
	// pull both lat & long from ingested json, convert into GeoPoint *obj
	locationLatitude, latOk := r.json.Path("data.location.latitude").Data().(float64)
	locationLongitude, longOk := r.json.Path("data.location.longitude").Data().(float64)
	if latOk && longOk {
		geoPoint := &latlng.LatLng{
			Latitude:  locationLatitude,
			Longitude: locationLongitude,
		}
		g = geoPoint
		return g, true
	} else {
		g = &latlng.LatLng{}
		return g, false
	}
}

// parse streaming report JSON packet for location heading
func (r *TransponderReportDataStreamV1) reportLocationHeading() (rlh float64, ok bool) {
	locationHeading, ok := r.json.Path("data.location.heading").Data().(float64)
	if ok {
		rlh = locationHeading
		return rlh, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for batteryVoltage
func (r *TransponderReportDataStreamV1) reportBatteryVoltage() (bv float64, ok bool) {
	paramsBatteryVoltage, ok := r.json.Path("data.parameters.batteryVoltage").Data().(float64)
	if ok {
		bv = paramsBatteryVoltage
		return bv, true
	} else {
		return 0, false
	}
}

// pase streaming report JSON packet for cellSignalStrength
func (r *TransponderReportDataStreamV1) reportCellSignalStrength() (css float64, ok bool) {
	paramsCellSignalStrength, ok := r.json.Path("data.parameters.cellSignalStrength").Data().(float64)
	if ok {
		css = paramsCellSignalStrength
		return css, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for isLowBattery
func (r *TransponderReportDataStreamV1) reportIsLowBattery() (islb bool, ok bool) {
	paramsIsLowBattery, ok := r.json.Path("data.parameters.isLowBattery").Data().(bool)
	if ok {
		islb = paramsIsLowBattery
		return islb, true
	} else {
		return false, false
	}
}

// parse streaming report JSON packet for odometer
func (r *TransponderReportDataStreamV1) reportOdometer() (odo float64, ok bool) {
	paramsOdometer, ok := r.json.Path("data.parameters.odometer").Data().(float64)
	if ok {
		odo = paramsOdometer
		return odo, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for speed
func (r *TransponderReportDataStreamV1) reportSpeed() (sp float64, ok bool) {
	paramsSpeed, ok := r.json.Path("data.parameters.speed").Data().(float64)
	if ok {
		sp = paramsSpeed
		return sp, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for speedLimit
func (r *TransponderReportDataStreamV1) reportSpeedLimit() (rsl float64, ok bool) {
	paramsSpeedLimit, ok := r.json.Path("data.parameters.speedLimit").Data().(float64)
	if ok {
		rsl = paramsSpeedLimit
		return rsl, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for reportTimestamp
func (r *TransponderReportDataStreamV1) reportDataTimestamp() (rdt time.Time, ok bool) {
	reportTimestamp, ok := r.json.Path("data.reportTimestamp").Data().(float64)
	if ok {
		rdt, _ = nanoEpochTimeObject(reportTimestamp)
		return rdt, true
	} else {
		return time.Time{}, false
	}
}

// parse streaming report JSON packet for transponder serial number
func (r *TransponderReportDataStreamV1) reportSerial() (s float64, ok bool) {
	serial, ok := r.json.Path("data.serial").Data().(float64)
	if ok {
		s = serial
		return s, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for geoTags array
// data.location.geoTags{account:{}, global:{}}
func (r *TransponderReportDataStreamV1) reportGeoTags() (t []GeoTagV1, ok bool) {
	// range over data.location.geoTags objects
	for tagSource, child := range r.json.Path("data.location.geoTags").ChildrenMap() {
		gt := GeoTagV1{}
		gt.TagSource = tagSource
		for tagName, meta := range child.ChildrenMap() {
			gt.TagName = tagName
			tagId, ok := meta.Path("geoTagId").Data().(float64)
			if !ok {
				return t, false
			}
			gt.GeoZoneId = tagId
			time, ok := meta.Path("timestamp").Data().(float64)
			if !ok {
				return t, false
			}
			gt.Timestamp, _ = nanoEpochTimeObject(time)
			t = append(t, gt) // add our tag to return array
		}
	}
	return t, true
}

// determine if the ReportDataStreamV1 packet we received is "valid"
func (r *TransponderReportDataStreamV1) validDataType() (ok bool) {
	dt := r.reportDataType
	if dt == "status" || dt == "parking" || dt == "stopped" || dt == "trip_report" || dt == "hard_accel" ||
		dt == "hard_braking" || dt == "hard_cornering" || dt == "overspeeding" || dt == "idling" {
		return true
	} else {
		return false
	}
}

// unpack a Transponder Report Data Stream V1 packet into a Firebase Transponder Event Report V1
func (r *TransponderReportDataStreamV1) firestoreRecord() (fbRecord FirestoreTransponderReportV1, err error) {
	if r.validDataType() {
		// attempt to pull data out of report packet, if we can't extract anything
		// it ends up as a nil value that will be ignored by Firestore at write time if
		// struct var has the "omitempty" tag in its firestore struct.

		configId, ok := r.reportConfigId()
		if ok {
			fbRecord.ConfigId = configId
		}
		reportDuration, ok := r.reportDuration()
		if ok {
			fbRecord.Duration = reportDuration
		}
		eventStart, ok := r.reportEventStart()
		if ok {
			fbRecord.EventStart = eventStart
		}
		reportInProg, ok := r.reportInProgress()
		if ok {
			fbRecord.InProgress = reportInProg
		}
		reportLocAcc, ok := r.reportLocationAccuracy()
		if ok {
			fbRecord.LocationAccuracy = reportLocAcc
		}
		reportLocHeading, ok := r.reportLocationHeading()
		if ok {
			fbRecord.Heading = reportLocHeading
		}
		reportGeoObj, ok := r.reportGeoObj()
		if ok {
			fbRecord.LatLng = reportGeoObj
		}
		reportBattVolt, ok := r.reportBatteryVoltage()
		if ok {
			fbRecord.BatteryVoltage = reportBattVolt
		}
		cellSignal, ok := r.reportCellSignalStrength()
		if ok {
			fbRecord.CellSignalStrength = cellSignal
		}
		isLowBatt, ok := r.reportIsLowBattery()
		if ok {
			fbRecord.IsLowBattery = isLowBatt
		}
		odometer, ok := r.reportOdometer()
		if ok {
			fbRecord.Odometer = odometer
		}
		speed, ok := r.reportSpeed()
		if ok {
			fbRecord.Speed = speed
		}
		speedLimit, ok := r.reportSpeedLimit()
		if ok {
			fbRecord.SpeedLimit = speedLimit
		}
		reportTimestamp, ok := r.reportDataTimestamp()
		if ok {
			fbRecord.ReportTimestamp = reportTimestamp
		}
		geoTags, ok := r.reportGeoTags()
		if ok {
			fbRecord.GeoTags = geoTags
		}
		serial, ok := r.reportSerial()
		if ok {
			fbRecord.Serial = serial
		}
		// reportDataType is unmarshalled and set earlier upstream
		fbRecord.Type = r.reportDataType
		// done forming new status report entry for firestore
		return fbRecord, nil
	} else {
		errMsg := fmt.Sprintf("ERROR: Unable to marshall streaming JSON record to FirestoreTransponderReportV1 struct: dataType is not compatible: %s:%s", r.reportType, r.reportDataType)
		log.Debugln(errMsg)
		return fbRecord, errors.New(errMsg)
	}
}
