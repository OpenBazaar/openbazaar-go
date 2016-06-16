package api

import "io"


// swagger:parameters status
type PeerIdParam struct {
    // IPNS id for the peer you're trying to reach.
    // eg: QmewaTzuA2gMjHyAGFN6wTWH7cVfZeApFM98TC28aSTy1P
    //
    // in: path
    // type: string
    // required: true
    PeerId string
}

// swagger:parameters putProfile
type ProfileParam struct {
    // Holds a Profile JSON object
    //
    // in: body
    // type: object
    // required: true
    Profile io.Reader
}


