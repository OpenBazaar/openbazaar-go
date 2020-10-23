package jwt

// Payload is a JWT payload according to the RFC 7519.
type Payload struct {
	Issuer         string   `json:"iss,omitempty"`
	Subject        string   `json:"sub,omitempty"`
	Audience       Audience `json:"aud,omitempty"`
	ExpirationTime *Time    `json:"exp,omitempty"`
	NotBefore      *Time    `json:"nbf,omitempty"`
	IssuedAt       *Time    `json:"iat,omitempty"`
	JWTID          string   `json:"jti,omitempty"`
}
