package main

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	log "github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/type/latlng"
)

// UNFINISHED!! https://track.carmasys.com:8343/browse/FIRE-2

func videoReportWriterV1(ctx context.Context, c *firestore.Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case rds := <-videoReportsV1:
			log.Debugf("videoReportWriterV1() received new report: %s:%s to process into Firestore...\n", rds.reportType, rds.reportDataType)
			// validate and populate our report struct
			// ok := rds.build() ...

			// build firestore reference
			ref := rds.firestoreReference(c)

			// marshall our video data streaming record into a firestore status record
			record, err := rds.firestoreRecord()
			if err != nil {
				log.Errorf("Unable to marshall streaming JSON report to a Firestore record: %s", rds.json.String())
				break
			}

			// check result
			result, err := ref.NewDoc().Set(ctx, record)
			if err != nil {
				log.Errorf("Firestore write error :%v", err)
			} else {
				log.Debugf("Firestore write result: %v", result)
			}
			// wait for more
		}
	}
}

// Methods on *VideorReportDataStreamV1
// These methods are intentionally left unversioned for now.

// create a firestore reference location on a V1 video report
// /account/{id}/vehicle/{cwWebId}/video_event/{UUID}
func (r *VideoReportDataStreamV1) firestoreReference(c *firestore.Client) *firestore.CollectionRef {
	ref := c.Collection("account").Doc(r.cwAccountId).Collection("vehicle").Doc(r.cwDeviceWebId).Collection("video_event")
	return ref
}

// parse a streaming report JSON packet for videoEventId
func (r *VideoReportDataStreamV1) videoEventId() (string, bool) {
	videoEventId, ok := r.json.Path("videoEventId").Data().(string)
	if ok {
		return videoEventId, true
	} else {
		return ``, false
	}
}

// parse a streaming report JSON packet for terminalNumber
func (r *VideoReportDataStreamV1) terminalNumber() (string, bool) {
	terminalNumber, ok := r.json.Path("videoEventMetadata.terminalNumber").Data().(string)
	if ok {
		return terminalNumber, true
	} else {
		return ``, false
	}
}

// parse a streaming report JSON packet for transponderId
func (r *VideoReportDataStreamV1) transponderId() (float64, bool) {
	transponderId, ok := r.json.Path("videoEventMetadata.transponderid").Data().(float64)
	if ok {
		return transponderId, true
	} else {
		return 0, false
	}
}

// parse a streaming report JSON packet for eventTimestamp
func (r *VideoReportDataStreamV1) eventTimestamp() (time.Time, bool) {
	eventTimestamp, ok := r.json.Path("videoEventMetadata.eventTimestamp").Data().(float64)
	if ok {
		// there's a float64 there as UTC epoch time. convert to time.Time
		es, _ := nanoEpochTimeObject(eventTimestamp)
		return es, true
	} else {
		return time.Time{}, false
	}
}

// parse a streaming report JSON packet for driver's username
func (r *VideoReportDataStreamV1) username() (string, bool) {
	username, ok := r.json.Path("videoEventMetadata.username").Data().(string)
	if ok {
		return username, true
	} else {
		return ``, false
	}
}

// parse a streaming report JSON packet for eventType
func (r *VideoReportDataStreamV1) eventType() (et []string, ok bool) {
	for _, a := range r.json.Path("videoEventMetadata.eventType").Children() {
		if IsNilInterface(a) {
			log.Debugf("NIL INTERFACE DETECTED LOL: VideoReportDataStreamV1.eventType(): %v", r.json.String())
			return et, false
		}
		et = append(et, a.Data().(string))
	}
	return et, true
}

// parse streaming report JSON packet location coordinates lat, long
// returns a "latlng.LatLng" pointer
func (r *VideoReportDataStreamV1) reportGeoObj() (g *latlng.LatLng, ok bool) {
	// pull both lat & long from ingested json, convert into GeoPoint *obj
	lat := "videoEventMetadata.kinematics.location.latitude"
	lon := "videoEventMetadata.kinematics.location.longitude"
	locationLatitude, latOk := r.json.Path(lat).Data().(float64)
	locationLongitude, longOk := r.json.Path(lon).Data().(float64)
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

// parse streaming report JSON packet for location accuracy key
func (r *VideoReportDataStreamV1) locationAccuracy() (rla float64, ok bool) {
	accuracy := "videoEventMetadata.kinematics.location.accuracy"
	locationAccuracy, ok := r.json.Path(accuracy).Data().(float64)
	if ok {
		rla = locationAccuracy
		return rla, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for speed
func (r *VideoReportDataStreamV1) speed() (sp float64, ok bool) {
	speed := "videoEventMetadata.kinematics.speed"
	paramsSpeed, ok := r.json.Path(speed).Data().(float64)
	if ok {
		sp = paramsSpeed
		return sp, true
	} else {
		return 0, false
	}
}

// parse streaming report JSON packet for location heading
func (r *VideoReportDataStreamV1) locationHeading() (rlh float64, ok bool) {
	heading := "videoEventMetadata.kinematics.heading"
	locationHeading, ok := r.json.Path(heading).Data().(float64)
	if ok {
		rlh = locationHeading
		return rlh, true
	} else {
		return 0, false
	}
}

// footageId from a single footage event from within a VideoReportDataStreamV1 object
// this is called by a higher-level function that iterates over []footage objects
func (r *VideoReportDataV1) footageId() (id string, ok bool) {
	username, ok := r.json.Path("footageId").Data().(string)
	if ok {
		return username, true
	} else {
		return ``, false
	}
}

// footage can be an array (ew) of objects if there's more than one camera
func (vr *VideoReportDataStreamV1) footage() (footage []VideoReportFootageV1, ok bool) {
	data := VideoReportDataV1{}
	for _, data.json = range vr.json.Path("footage").Children() {
		entry := VideoReportFootageV1{} // init our footage entry struct
		// begin identifying available data and packing it into entry
		footageId, ok := data.footageId()
		if ok {
			entry.FootageId = footageId
		}

		// all populated, append to our array before moving on to next footage entry
		footage = append(footage, entry)
	}
	/*
		footageId, ok := vr.footageId()
		if ok {
			fbRecord.FootageId = footageId
		}
		garminCamName, ok := vr.garminCameraName()
		if ok {
			fbRecord.GarminCameraName = garminCamName
		}
		garminFootageFile, ok := vr.garminFootageFilename()
		if ok {
			fbRecord.GarminFootageFilename = garminFootageFile
		}
		preview, ok := vr.preview()
		if ok {
			fbRecord.Preview = preview
		}
		footageTs, ok := vr.footageTimestamp()
		if ok {
			fbRecord.FootageTimestamp = footageTs
		}
		footageDur, ok := vr.footageDuration()
		if ok {
			fbRecord.FootageDuration = footageDur
		}
		footageAudioInc, ok := vr.footageAudioInc()
		if ok {
			fbRecord.FootageAudioIncluded = footageAudioInc
		}
		size, ok := vr.size()
		if ok {
			fbRecord.Size = size
		}
		md5, ok := vr.md5()
		if ok {
			fbRecord.Md5 = md5
		}
		crc32c, ok := vr.crc32c()
		if ok {
			fbRecord.Crc32c = crc32c
		}
		contentType, ok := vr.contentType()
		if ok {
			fbRecord.ContentType = contentType
		}
		footagePath, ok := vr.footagePath()
		if ok {
			fbRecord.FootagePath = footagePath
		}
		uploadStatus, ok := vr.uploadStatus()
		if ok {
			fbRecord.UploadStatus = uploadStatus
		}
		uploadStartTs, ok := vr.uploadStartTimestamp()
		if ok {
			fbRecord.UploadStartTime = uploadStartTs
		}
		lastChunkRx, ok := vr.lastChunkRxTimestamp()
		if ok {
			fbRecord.LastChunkRxTime = lastChunkRx
		}
		bytesRemaining, ok := vr.bytesRemaining()
		if ok {
			fbRecord.BytesRemaining = bytesRemaining
		}
		uploadStatusReason, ok := vr.uploadStatusReason()
		if ok {
			fbRecord.UploadStatusReason = uploadStatusReason
		}
	*/
	return footage, ok
}

func (vr *VideoReportDataStreamV1) firestoreRecord() (fbRecord FirestoreVideoReportV1, err error) {
	// video event metadata
	videoEventId, ok := vr.videoEventId()
	if ok {
		fbRecord.VideoEventId = videoEventId
	}
	terminalNumber, ok := vr.terminalNumber()
	if ok {
		fbRecord.TerminalNumber = terminalNumber
	}
	transponderId, ok := vr.transponderId()
	if ok {
		fbRecord.TransponderId = transponderId
	}
	evTs, ok := vr.eventTimestamp()
	if ok {
		fbRecord.EventTimestamp = evTs
	}
	username, ok := vr.username()
	if ok {
		fbRecord.Username = username
	}
	eventType, ok := vr.eventType()
	if ok {
		fbRecord.EventType = eventType
	}
	location, ok := vr.reportGeoObj()
	if ok {
		fbRecord.Location = location
	}
	locationAcc, ok := vr.locationAccuracy()
	if ok {
		fbRecord.LocationAccuracy = locationAcc
	}
	speed, ok := vr.speed()
	if ok {
		fbRecord.Speed = speed
	}
	heading, ok := vr.locationHeading()
	if ok {
		fbRecord.Heading = heading
	}
	// footage object array
	footageEntries, ok := vr.footage()
	if ok {
		fbRecord.Footage = footageEntries
	}

	return fbRecord, nil
}
