# Firestream App

This is a CLAPI stream adapter written in Golang. Its purpose is to stream CLAPI
data of interest into GCP Firestore db. The intent is to allow web/app clients
to see nearly-live vehicle data overlaid on top of maps.

The app intends to save resources by reducing polling of our web / native apps
into the CLAPI for transponder status information.

Its intent is to always be connected to the CLAPI stream and receiving transponder
data.

There is a companion cli app to replay the data inserted into firebase by this webservice.
This app is called [replayfirestream](https://github.com/kevholmes/replayfirestream) and
can be useful for testing or simulating bugs by users in the new ios/android app that listen
to firestore endpoints for live updates.
The data is inserted by replayfirestream at the same rate it was reported live by the transponder, but with
spoofed timestamps to simulate the live experience.

## Basic functionality

The Firestream adapter will init a connection to the CLAP websocket for an env_all
endpoint stream with all transponders available being sent thru this connection.

At startup the stream adapter will build a map of all accounts and associated webIds.
This will help us keep track of the accountId and cwWebid context that is needed
when writing our stream data to Firestore. If we do not have a hit within our accountId:TransponderId
map for a streaming point that has arrived, we query Navajo for any info about this unit.
If we can't find out accountId and webId for the transponder, we log that serial # and
occurance time to metrics interface. If we do get a hit for the transponder, we will
update our mapping of who owns what.

These help us keep track of what belongs to who:
map[webId]accountId
map[transponderId]webId

Once the initial navajo map has been built, we then attempt to connect
to the streaming CLAPI and upgrade to a websocket.

When the websocket is successfully opened, we begin processing data. We are only
interested in a limited set of this streaming JSON data.

When that match happens we make our write to Firestore and do not specify a document id.
Firestore will automatically create our document with a unique name field using the
parameters of interest we have pulled out of the status packet JSON data.

## Evironment vars

To configure Firestream envionment variables are the way to go:

```bash
export CLAPI_HOST=http://devpush0:8080/v2/open_stream/local_all
export CLAPI_WSHOST=ws://devpush0:8080/v2/open_stream/local_all
export CLAPI_KEY=anOauthConsumerKey
export CLAPI_SEC=anOauthSecretKey

export NAVAJO_URL=http://172.20.0.51:8029
export NAVAJO_USER=""
export NAVAJO_PW=aNavajoPassword

export GOOGLE_APPLICATION_CREDENTIALS=aServiceAccountAuthFile.json
export GOOGLE_PROJECT_ID=dev-latinum
# firestore emulator
export FIRESTORE_EMULATOR_HOST="localhost:8070"
```

These values are provided automatically to the docker container when running in dev etc.
Ansible plays supply a {environment}.env file for any VMs registered to run Firestream,
with all required values populated by default. This file is placed in the same area
as the container's docker-compose.yml file at deploy time, so a docker-compose up
command will pick up the .env file and you're off to the races.

## Supported Reports

Currently we support reports utilizing the "type" tag as REPORT_DATA_EVENT_TYPE.

stopped report:

```json
// https://api.carmalink.com/v2/transponder/519372/data/stopped
{
    "serial": 519372,
    "type": "stopped",
    "configId": 473123,
    "eventStart": 1613577074755,
    "reportTimestamp": 1613577074755,
    "duration": 0,
    "inProgress": true,
    "location": {
        "latitude": 41.418956099999996,
        "longitude": -70.58776809999999,
        "accuracy": 4.5542,
        "heading": 284.9672
    },
    "parameters": {
        "cellSignalStrength": -51.0,
        "speed": 0.0,
        "batteryVoltage": 12.33794,
        "isLowBatteryVoltage": false
    },
    "functions": {}
}
```

parking report:

```json
// https://api.carmalink.com/v2/transponder/519372/data/parking
{
    "serial": 519372,
    "type": "parking",
    "configId": 473119,
    "eventStart": 1613577228453,
    "reportTimestamp": 1613577228453,
    "duration": 0,
    "inProgress": true,
    "location": {
        "latitude": 41.418956099999996,
        "longitude": -70.58776809999999,
        "accuracy": 1.243,
        "heading": 284.9672
    },
    "parameters": {
        "cellSignalStrength": -51.0,
        "speed": 0.0,
        "batteryVoltage": 12.369361,
        "isLowBatteryVoltage": false
    },
    "functions": {
        "average": {
            "batteryVoltage": 12.369361
        }
    }
}
```

status aka vehicle_status:

```json
{
    "serial": 519372,
    "type": "vehicle_status",
    "configId": 473122,
    "eventStart": 1613576740322,
    "reportTimestamp": 1613577228453,
    "duration": 488131,
    "inProgress": false,
    "location": {
        "latitude": 41.418956099999996,
        "longitude": -70.58776809999999,
        "accuracy": 1.243,
        "heading": 284.9672
    },
    "parameters": {
        "cellSignalStrength": -51.0,
        "speed": 0.0,
        "batteryVoltage": 12.369361,
        "isLowBatteryVoltage": false
    },
    "functions": {
        "integrate": {
            "speed": 3.0439425
        }
    }
}
```

VIDEO_EVENT_TYPE utilizes a single type of predictable structure for its's reports sent
down the stream pipeline:

```json
{
    "videoEventId": "4ad20507",
    "videoEventMetadata": {
        "terminalNumber": "1.2738.3955638690",
        "transponderId": 519521,
        "username": "chris",
        "eventTimestamp": 1613858373956,
        "eventType": [
            "TRANSPONDER_STOPPED"
        ],
        "kinematics": {
            "location": {
                "latitude": 42.6266232,
                "longitude": -73.8748708,
                "accuracy": 9.0460005
            },
            "speed": 0.0,
            "heading": 86.74167
        }
    },
    "footage": [
        {
            "footageId": "b042ff1af6c6ac40a5a125de5885cfc9",
            "footageMetadata": {
                "garminCameraName": "0",
                "garminFootageFileName": "snapshot-0-1613858375080",
                "isPreview": false,
                "timestamp": 1613858375080,
                "duration": 0,
                "isAudioIncluded": false,
                "size": 88206,
                "md5": "sEL/GvbGrECloSXeWIXPyQ==",
                "crc32c": "l1HupQ==",
                "contentType": "image/jpeg"
            },
            "privateFilePath": "gs://dev-carmalinkapi-dashcamfootage/b042ff1af6c6ac40a5a125de5885cfc9/snapshot-0-1613858375080",
            "uploadState": {
                "status": "COMPLETED",
                "startTimestamp": 1613858377070,
                "lastChunkRxTimestamp": 1613858377568,
                "bytesRemaining": 0,
                "reason": "Success"
            }
        }
    ]
}
```

ELD_RECORD_TYPE is more complex and has a number of possible reports sent down the pipe,
and not all have an associated transponder or driver id (records could be updated via Ultra (external website/app.)

## Resource Conservation - Concept Only

Rather than writing any and all data into Firestore when it arrives via the
CLAPI websocket stream the app will take a balanced approach to conserve
resources and reduce cloud spending when not necessary.

The adapter will have a configurable value INACTIVE_RATE that determines
how often a new status location for an active transponder is written into
Firestore when a client isn't actively listening. This is to ensure
we have pretty recent data available immediately upon a client pulling
up their live fleet map.

Clients will be expected to write to a firestore document outlining their
expectations for live data. The Firestream adapter will use these documents
to determine how to proceed with updating live transponder in Firestore.

The document will tell the Firestream adapter what transponders the client
is interested in for streaming data.

```json
// app/web clients update these two tags:
transponders: Array[]
clientRequestTime: Date and Time
clientId: String // some way to know what unique client instance/uuid made request. tie into existing auth document with uid there?
// Firestream adapter updates this:
streamTurndownTime: Date and Time
firestreamOK: Boolean
```

The transponders tag is an array of serial numbers the client is interested in.

clientRequestTime is a Date and Time Firebase Type in microseconds. This is a
time that the client must maintain and keep updating as it continues listening.

This way if a client leaves the site - the clientRequestTime will not be updated
and Firestream can eventually time out this live data request and slow itself
down to INACTIVE_RATE.

Firestream upon detecting a valid and recent stream request from a client will
update streamTurndownTime with a time in the future. This time is when it will
mark the request as no longer active and turn the update rate back down to
INACTIVE_RATE. The streamTurndownTime is set by environment variable with
the name TURNDOWN_TIME and it is set in milliseconds.

The firestreamOK boolean is there to let clients know if firestream is
working properly. If this value is false there is likely a configuration,
permission, or GCP platform error that needs some kind of intervention.