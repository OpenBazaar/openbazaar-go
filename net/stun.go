package net

import (
	"errors"
	"fmt"
	"math/rand"

	stunlib "github.com/ccding/go-stun/stun"
	"github.com/op/go-logging"
)

var STUN_SERVERS = []string{
	"stun.ekiga.net",
	"stun.ideasip.com",
	"stun.voiparound.com",
	"stun.voipbuster.com",
	"stun.voipstunt.com",
	"stun.voxgratia.org",
}

var STUN_PORT = 3478

var log = logging.MustGetLogger("stun")

const _NATType_name = "NAT_ERRORNAT_UNKNOWNNAT_NONENAT_BLOCKEDNAT_FULLNAT_SYMETRICNAT_RESTRICTEDNAT_PORT_RESTRICTEDNAT_SYMETRIC_UDP_FIREWALL"

var _NATType_index = [...]uint8{0, 9, 20, 28, 39, 47, 59, 73, 92, 117}

var errStunFailed = errors.New("exhausted list of stun servers")

func Stun() (int, error) {
	Shuffle(STUN_SERVERS)
	for _, server := range STUN_SERVERS {
		client := stunlib.NewClient()
		client.SetServerHost(server, STUN_PORT)
		nat, host, err := client.Discover()
		if err == nil {
			log.Infof("%s on UDP %s:%d", NATtoString(nat), host.IP(), host.Port())
			return int(host.Port()), nil
		}
	}
	return 0, errStunFailed
}

func NATtoString(i stunlib.NATType) string {
	if i < 0 || i >= stunlib.NATType(len(_NATType_index)-1) {
		return fmt.Sprintf("NATType(%d)", i)
	}
	return _NATType_name[_NATType_index[i]:_NATType_index[i+1]]
}

func Shuffle(a []string) {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}
}
