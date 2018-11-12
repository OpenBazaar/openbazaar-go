package main

import (
	"log"

	"gx/ipfs/QmVwfv63beSAAirq3tiLY6fkNqvjThLnCofL7363YkaScy/goupnp/ssdp"
)

func main() {
	c := make(chan ssdp.Update)
	srv, reg := ssdp.NewServerAndRegistry()
	reg.AddListener(c)
	go listener(c)
	if err := srv.ListenAndServe(); err != nil {
		log.Print("ListenAndServe failed: ", err)
	}
}

func listener(c <-chan ssdp.Update) {
	for u := range c {
		if u.Entry != nil {
			log.Printf("Event: %v USN: %s Entry: %#v", u.EventType, u.USN, *u.Entry)
		} else {
			log.Printf("Event: %v USN: %s Entry: <nil>", u.EventType, u.USN)
		}
	}
}
