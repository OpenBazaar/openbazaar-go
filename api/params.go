package api


// swagger:parameters status
type PeerIdParam struct {
    //
    // IPNS id for the peer you're trying to reach.
    // eg: QmewaTzuA2gMjHyAGFN6wTWH7cVfZeApFM98TC28aSTy1P
    //
    // in: path
    // schema: object
    // required: true
    PeerId string
}

// ProfileModel represents a profile object
//
// A profile holds information about peers on the network.
//
// swagger:model ProfileModel
type ProfileModel struct {
    // Name of the user
    // required: true
    Name string `json:"name"`

    // User handle
    // Either IPNS id or Blockchain ID (i.e. @OpenBazaar)
    // required: true
    Handle string `json:"handle"`

    // About description of the peer
    // required: true
    About string `json:"about"`

    // Email address
    // required: true
    Email string `json:"email"`

    // Location
    // required: true
    Location string `json:"location"`

    // NSFW status of the peer
    // required: true
    NSFW string `json:"nsfw"`

    // Short Description
    // required: true
    ShortDescription string `json:"short_description"`

    // Vendor status of peer
    // required: true
    Vendor string `json:"vendor"`

    // URL of the user's website
    // required: true
    Website string `json:"website"`
}

// swagger:parameters putProfile
type ProfileParam struct {
    // Holds a Profile JSON object
    //
    // in: body
    // type: object
    // required: true
    Profile ProfileModel
}

// A ProfileResponse is the response for Profile calls
// swagger:response ProfileResponse
type ProfileResponse struct {
	// Response from server
	// in: body
	Body struct {
		// Success Status of true or false
		//
		// Required: true
		Success string `json:"success"`
		// An optional reason if there is a failure or error
		Reason string `json:"reason"`
	}
}

// A StatusResponse is the response for Status calls
// swagger:response StatusResponse
type StatusResponse struct {
	// Response from server
	// in: body
	Body struct {
		// Success Status of true or false
		//
		// Required: true
		Status string `json:"status"`
	}
}
