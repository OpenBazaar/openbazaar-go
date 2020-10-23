package jwt_test

import (
	"testing"
	"time"

	"github.com/gbrlsnchs/jwt/v3"
)

var (
	benchHS256 = jwt.NewHS256([]byte("secret"))
	benchRecv  []byte
)

func BenchmarkSign(b *testing.B) {
	now := time.Now()
	var (
		token []byte
		err   error
		pl    = jwt.Payload{
			Issuer:         "gbrlsnchs",
			Subject:        "someone",
			Audience:       jwt.Audience{"https://golang.org", "https://jwt.io"},
			ExpirationTime: jwt.NumericDate(now.Add(24 * 30 * 12 * time.Hour)),
			NotBefore:      jwt.NumericDate(now.Add(30 * time.Minute)),
			IssuedAt:       jwt.NumericDate(now),
			JWTID:          "foobar",
		}
	)
	b.Run("Default", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			token, err = jwt.Sign(pl, benchHS256)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run(`With "kid"`, func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			token, err = jwt.Sign(pl, benchHS256, jwt.KeyID("kid"))
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run(`With "cty" and "kid"`, func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			token, err = jwt.Sign(pl, benchHS256, jwt.ContentType("cty"), jwt.KeyID("kid"))
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	benchRecv = token

}

func BenchmarkVerify(b *testing.B) {
	var (
		token = []byte(
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
				"eyJpc3MiOiJnYnJsc25jaHMiLCJzdWIiOiJzb21lb25lIiwiYXVkIjpbImh0dHBzOi8vZ29sYW5nLm9yZyIsImh0dHBzOi8vand0LmlvIl0sImV4cCI6MTU5MzM5MTE4MiwibmJmIjoxNTYyMjg4OTgyLCJpYXQiOjE1NjIyODcxODIsImp0aSI6ImZvb2JhciJ9." +
				"bKevp7jmMbH9-Hy5g5OxLgq8tg13z9voH7lZ4m9y484",
		)
		err error
	)
	b.Run("Default", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			var pl jwt.Payload
			if _, err = jwt.Verify(token, benchHS256, &pl); err != nil {
				b.Fatal(err)
			}
		}
	})
}
