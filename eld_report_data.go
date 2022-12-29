package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	log "github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/type/latlng"
)

// Handle inbound structs from upstream report router
func eldReportWriterV1(ctx context.Context, c *firestore.Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case rds := <-eldReportsV1:
			log.Debugf("eldReportWriterV1() received new report: %s:%s to process into Firestore...\n", rds.reportType, rds.reportDataType)

			// we are writing only navigation records for now, don't want to see warnings
			// from failed build() method calls on ELD stream records
			if rds.reportDataType != "navigation" {
				break // move on
			}

			// validate and populate our report struct
			ok := rds.build()
			if !ok {
				log.Warnf("Unable to validate and build a TransponderReportDataStreamV1 object sent from upstream channel transponderReportsV1\n")
				break // unable to validate the packet, drop it and move on
			}

			// build firestore reference
			ref := rds.firestoreReference(c)

			// marshall our eld data streaming record into a firestore record
			record, err := rds.firestoreRecord()
			if err != nil {
				log.Errorf("Unable to marshall streaming JSON report to Firestore record: %s", rds.json.String())
				break // don't write potentially bad data to Firestore
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

// Methods on *EldReportDataStreamV1
// Intentionally not versioned for now

// create a firestore reference location on a V1 eld data report
func (r *EldReportDataStreamV1) firestoreReference(c *firestore.Client) *firestore.CollectionRef {
	ref := c.Collection("account").Doc(r.cwAccountId).Collection("driver").Doc(r.clUserId).Collection("report_data")
	return ref
}

// validate/populate all items needed for an actionable EldReportDataStreamV1 type
func (r *EldReportDataStreamV1) build() (ok bool) {
	// eld reports require an accountId and driver id
	clApiAcctId, aOk := r.json.Path("accountId").Data().(float64)
	clApiDrivId, dOk := r.json.Path("data.userId").Data().(float64)
	if !aOk || !dOk {
		log.Warnf("EldReportDataStreamV1.build(): report doesn't contain required value(s): accountId:%t, data.userId:%t\n", aOk, dOk)
		return false
	}
	accountId := fmt.Sprintf("%.0f", clApiAcctId)
	userId := fmt.Sprintf("%.0f", clApiDrivId)
	// get our clapi <-> cartwheel accountId match out of global map (mutex used)
	ok = r.cartwheelMap(accountId)
	if !ok {
		log.Warnf("EldReportDataStreamV1.build(): Unable to obtain cartwheel accountId from in-memory mapping (accountId): %v\n", accountId)
		return false
	}
	// assign userId (not from cartwheel)
	r.clUserId = userId
	return true
}

// fetch / insert cartwheel ids from in-memory map into our record
func (r *EldReportDataStreamV1) cartwheelMap(a string) (ok bool) {
	navajoReferenceIds.mutex.Lock()
	defer navajoReferenceIds.mutex.Unlock()
	// check global struct NavajoReferenceIds for matches
	cwAccountId, acctOk := navajoReferenceIds.clAccountIdMap[a]
	if !acctOk {
		log.Warnf("getCartwheelIds() unable to find match for CL API AccountId (%v) : Navajo Account Id\n", a)
		return false
	}
	r.cwAccountId = cwAccountId
	return true
}

// determine if the EldReportDataStreamV1 packet we received is "valid"
func (r *EldReportDataStreamV1) validDataType() (ok bool) {
	dt := r.reportDataType
	if dt == "navigation" {
		return true
	} else {
		return false
	}
}

// parse a streaming ELD report JSON packet for usDotNumber
func (r *EldReportDataStreamV1) usDotNum() (u string, ok bool) {
	usDotNum, ok := r.json.Path("usDotNumber").Data().(string)
	if ok {
		u = usDotNum
		return u, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for userId
func (r *EldReportDataStreamV1) userId() (u float64, ok bool) {
	userId, ok := r.json.Path("userId").Data().(float64)
	if ok {
		u = userId
		return u, true
	} else {
		return 0, false
	}
}

// parse a streaming ELD report JSON packet for username
func (r *EldReportDataStreamV1) username() (u string, ok bool) {
	username, ok := r.json.Path("userName").Data().(string)
	if ok {
		u = username
		return u, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for transponderId
func (r *EldReportDataStreamV1) transponderId() (t float64, ok bool) {
	tId, ok := r.json.Path("sentFrom.transponderId").Data().(float64)
	if ok {
		t = tId
		return t, true
	} else {
		return 0, false
	}
}

// parse a streaming ELD report JSON packet for terminalNumber
func (r *EldReportDataStreamV1) terminalNumber() (t string, ok bool) {
	tNum, ok := r.json.Path("sentFrom.terminalNumber").Data().(string)
	if ok {
		t = tNum
		return t, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for serverRxTimestamp
func (r *EldReportDataStreamV1) serverRxTimestamp() (t time.Time, ok bool) {
	rx, ok := r.json.Path("sentFrom.serverRxTimestamp").Data().(float64)
	if ok {
		// convert UTC epoch millis to time.Time
		t, _ := nanoEpochTimeObject(rx)
		return t, true
	} else {
		return time.Time{}, false
	}
}

// parse a streaming ELD report JSON packet for eventId
func (r *EldReportDataStreamV1) eventId() (e string, ok bool) {
	eId, ok := r.json.Path("eventId").Data().(string)
	if ok {
		e = eId
		return e, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for recordId
func (r *EldReportDataStreamV1) recordId() (record string, ok bool) {
	rId, ok := r.json.Path("recordId").Data().(string)
	if ok {
		record = rId
		return record, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for recordTimestamp
func (r *EldReportDataStreamV1) recordTimestamp() (t time.Time, ok bool) {
	rTime, ok := r.json.Path("recordTimestamp").Data().(float64)
	if ok {
		// convert UTC epoch millis to time.Time
		t, _ := nanoEpochTimeObject(rTime)
		return t, true
	} else {
		return time.Time{}, false
	}
}

// parse a streaming ELD report JSON packet for recordStatus
func (r *EldReportDataStreamV1) recordStatus() (s string, ok bool) {
	recordStatus, ok := r.json.Path("recordStatus").Data().(string)
	if ok {
		s = recordStatus
		return s, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for recordOrigin
func (r *EldReportDataStreamV1) recordOrigin() (s string, ok bool) {
	recordOrigin, ok := r.json.Path("recordOrigin").Data().(string)
	if ok {
		s = recordOrigin
		return s, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for eventStartTimestamp
func (r *EldReportDataStreamV1) eventStartTimestamp() (t time.Time, ok bool) {
	est, ok := r.json.Path("recordData.eventStartTimestamp").Data().(float64)
	if ok {
		t, _ := nanoEpochTimeObject(est)
		return t, true
	} else {
		return time.Time{}, false
	}
}

// parse a streaming ELD report JSON packet for eventEndTimestamp
func (r *EldReportDataStreamV1) eventEndTimestamp() (t time.Time, ok bool) {
	eet, ok := r.json.Path("recordData.eventEndTimestamp").Data().(float64)
	if ok {
		t, _ := nanoEpochTimeObject(eet)
		return t, true
	} else {
		return time.Time{}, false
	}
}

// parse a streaming ELD report JSON packet for navigationEvent
func (r *EldReportDataStreamV1) navigationEvent() (n string, ok bool) {
	ne, ok := r.json.Path("recordData.navigationEvent").Data().(string)
	if ok {
		n = ne
		return n, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for vehicleMode
func (r *EldReportDataStreamV1) vehicleMode() (m string, ok bool) {
	vm, ok := r.json.Path("recordData.vehicleMode").Data().(string)
	if ok {
		m = vm
		return m, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for locationType
func (r *EldReportDataStreamV1) locationType() (t string, ok bool) {
	lc, ok := r.json.Path("recordData.locationType").Data().(string)
	if ok {
		t = lc
		return t, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for location: lat, lng
func (r *EldReportDataStreamV1) location() (l *latlng.LatLng, ok bool) {
	// pull both lat & long from ingested json, convert into GeoPoint *obj
	locationLatitude, latOk := r.json.Path("recordData.location.latitude").Data().(float64)
	locationLongitude, longOk := r.json.Path("recordData.location.longitude").Data().(float64)
	if latOk && longOk {
		geoPoint := &latlng.LatLng{
			Latitude:  locationLatitude,
			Longitude: locationLongitude,
		}
		l = geoPoint
		return l, true
	} else {
		l = &latlng.LatLng{}
		return l, false
	}
}

// parse a streaming ELD report JSON packet for geoDescription
func (r *EldReportDataStreamV1) geoDescription() (g string, ok bool) {
	gd, ok := r.json.Path("recordData.location.geoDescription").Data().(string)
	if ok {
		g = gd
		return g, true
	} else {
		return ``, false
	}
}

// parse a streaming ELD report JSON packet for meters
func (r *EldReportDataStreamV1) meters() (m float64, ok bool) {
	meters, ok := r.json.Path("recordData.meters").Data().(float64)
	if ok {
		m = meters
		return m, true
	} else {
		return 0, false
	}
}

// parse a streaming ELD report JSON packet for isDiagnosticActive
func (r *EldReportDataStreamV1) isDiagnosticActive() (a bool, ok bool) {
	active, ok := r.json.Path("isDiagnosticActive").Data().(bool)
	if ok {
		a = active
		return a, true
	} else {
		return false, false
	}
}

// parse a streaming ELD report JSON packet for isMalfunctionActive
func (r *EldReportDataStreamV1) isMalfunctionActive() (a bool, ok bool) {
	active, ok := r.json.Path("isMalfunctionActive").Data().(bool)
	if ok {
		a = active
		return a, true
	} else {
		return false, false
	}
}

// unpack an EldReportDataStream V1 packet into a Firebase Eld Report V1
func (r *EldReportDataStreamV1) firestoreRecord() (fbRecord FirestoreEldReportV1, err error) {
	if r.validDataType() {
		usDotNum, ok := r.usDotNum()
		if ok {
			fbRecord.UsDotNumber = usDotNum
		}
		userId, ok := r.userId()
		if ok {
			fbRecord.UserId = userId
		}
		username, ok := r.username()
		if ok {
			fbRecord.Username = username
		}
		transponderId, ok := r.transponderId()
		if ok {
			fbRecord.TransponderId = transponderId
		}
		terminalNum, ok := r.terminalNumber()
		if ok {
			fbRecord.TerminalNumber = terminalNum
		}
		serverRxTimestamp, ok := r.serverRxTimestamp()
		if ok {
			fbRecord.ServerRxTimestamp = serverRxTimestamp
		}
		eventId, ok := r.eventId()
		if ok {
			fbRecord.EventId = eventId
		}
		recordId, ok := r.recordId()
		if ok {
			fbRecord.RecordId = recordId
		}
		recordTimestamp, ok := r.recordTimestamp()
		if ok {
			fbRecord.RecordTimestamp = recordTimestamp
		}
		recordStatus, ok := r.recordStatus()
		if ok {
			fbRecord.RecordStatus = recordStatus
		}
		recordOrigin, ok := r.recordOrigin()
		if ok {
			fbRecord.RecordOrigin = recordOrigin
		}
		eventStartTimestamp, ok := r.eventStartTimestamp()
		if ok {
			fbRecord.EventStartTimestamp = eventStartTimestamp
		}
		eventEndTimestamp, ok := r.eventEndTimestamp()
		if ok {
			fbRecord.EventEndTimestamp = eventEndTimestamp
		}
		navEvent, ok := r.navigationEvent()
		if ok {
			fbRecord.NavigationEvent = navEvent
		}
		vehicleMode, ok := r.vehicleMode()
		if ok {
			fbRecord.VehicleMode = vehicleMode
		}
		locationType, ok := r.locationType()
		if ok {
			fbRecord.LocationType = locationType
		}
		location, ok := r.location()
		if ok {
			fbRecord.Location = location
		}
		geoDesc, ok := r.geoDescription()
		if ok {
			fbRecord.GeoDescription = geoDesc
		}
		meters, ok := r.meters()
		if ok {
			fbRecord.Meters = meters
		}
		isDiagActive, ok := r.isDiagnosticActive()
		if ok {
			fbRecord.IsDiagnosticActive = isDiagActive
		}
		isMalActive, ok := r.isMalfunctionActive()
		if ok {
			fbRecord.IsMalfunctionActive = isMalActive
		}
		fbRecord.Type = r.reportDataType
		return fbRecord, nil
	} else {
		errMsg := fmt.Sprintf("ERROR: Unable to marshall streaming JSON record to FirestoreTransponderReportV1 struct: dataType is not compatible: %s:%s", r.reportType, r.reportDataType)
		log.Debugln(errMsg)
		return fbRecord, errors.New(errMsg)
	}
}
