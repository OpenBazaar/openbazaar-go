// +build windows

package core

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cretz/bine/tor"
	"golang.org/x/net/context"
)

func StartNativeTor() (int, error) {
	fmt.Println("Starting Tor controller, please wait...")
	t, err = tor.Start(context.TODO(), nil)

	if err != nil {
		log.Panicf("Unable to start Tor: %v", err)
	}
	defer t.Close()

	listenCtx, listenCancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer listenCancel()

	dialer, err := t.Dialer(listenCtx, nil)
	if err != nil {
		return 0, err
	}

	httpClient := &http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}
	// Get /
	resp, err := httpClient.Get("http://my7nrnmkscxr32zo.onion/verified_moderators")
	if err != nil {
		return 0, err
	}
	fmt.Println(resp)
	defer resp.Body.Close()

	return 1, nil
}
