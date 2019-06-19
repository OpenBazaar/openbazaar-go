package multiaddr

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func newMultiaddr(t *testing.T, a string) Multiaddr {
	m, err := NewMultiaddr(a)
	if err != nil {
		t.Error(err)
	}
	return m
}

func TestConstructFails(t *testing.T) {
	cases := []string{
		"/ip4",
		"/ip4/::1",
		"/ip4/fdpsofodsajfdoisa",
		"/ip6",
		"/ip6zone",
		"/ip6zone/",
		"/ip6zone//ip6/fe80::1",
		"/udp",
		"/tcp",
		"/sctp",
		"/udp/65536",
		"/tcp/65536",
		"/quic/65536",
		"/onion/9imaq4ygg2iegci7:80",
		"/onion/aaimaq4ygg2iegci7:80",
		"/onion/timaq4ygg2iegci7:0",
		"/onion/timaq4ygg2iegci7:-1",
		"/onion/timaq4ygg2iegci7",
		"/onion/timaq4ygg2iegci@:666",
		"/onion3/9ww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80",
		"/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd7:80",
		"/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:0",
		"/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:-1",
		"/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd",
		"/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyy@:666",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA7:80",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA:0",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA:0",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA:-1",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA@:666",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA7:80",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA:0",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA:0",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA:-1",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA@:666",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzu",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzu77",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzu:80",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq:-1",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzu@",
		"/udp/1234/sctp",
		"/udp/1234/udt/1234",
		"/udp/1234/utp/1234",
		"/ip4/127.0.0.1/udp/jfodsajfidosajfoidsa",
		"/ip4/127.0.0.1/udp",
		"/ip4/127.0.0.1/tcp/jfodsajfidosajfoidsa",
		"/ip4/127.0.0.1/tcp",
		"/ip4/127.0.0.1/quic/1234",
		"/ip4/127.0.0.1/ipfs",
		"/ip4/127.0.0.1/ipfs/tcp",
		"/ip4/127.0.0.1/p2p",
		"/ip4/127.0.0.1/p2p/tcp",
		"/unix",
		"/ip4/1.2.3.4/tcp/80/unix",
		"/ip4/127.0.0.1/tcp/9090/http/p2p-webcrt-direct",
		"/",
		"",
	}

	for _, a := range cases {
		if _, err := NewMultiaddr(a); err == nil {
			t.Errorf("should have failed: %s - %s", a, err)
		}
	}
}

func TestEmptyMultiaddr(t *testing.T) {
	_, err := NewMultiaddrBytes([]byte{})
	if err == nil {
		t.Fatal("should have failed to parse empty multiaddr")
	}
}

func TestConstructSucceeds(t *testing.T) {
	cases := []string{
		"/ip4/1.2.3.4",
		"/ip4/0.0.0.0",
		"/ip6/::1",
		"/ip6/2601:9:4f81:9700:803e:ca65:66e8:c21",
		"/ip6/2601:9:4f81:9700:803e:ca65:66e8:c21/udp/1234/quic",
		"/ip6zone/x/ip6/fe80::1",
		"/ip6zone/x%y/ip6/fe80::1",
		"/ip6zone/x%y/ip6/::",
		"/ip6zone/x/ip6/fe80::1/udp/1234/quic",
		"/onion/timaq4ygg2iegci7:1234",
		"/onion/timaq4ygg2iegci7:80/http",
		"/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:1234",
		"/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80/http",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA/http",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA/udp/8080",
		"/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA/tcp/8080",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuqzwas",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuqzwassw",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq/http",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq/tcp/8080",
		"/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq/udp/8080",
		"/udp/0",
		"/tcp/0",
		"/sctp/0",
		"/udp/1234",
		"/tcp/1234",
		"/sctp/1234",
		"/udp/65535",
		"/tcp/65535",
		"/ipfs/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC",
		"/p2p/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC",
		"/udp/1234/sctp/1234",
		"/udp/1234/udt",
		"/udp/1234/utp",
		"/tcp/1234/http",
		"/tcp/1234/https",
		"/ipfs/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC/tcp/1234",
		"/p2p/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC/tcp/1234",
		"/ip4/127.0.0.1/udp/1234",
		"/ip4/127.0.0.1/udp/0",
		"/ip4/127.0.0.1/tcp/1234",
		"/ip4/127.0.0.1/tcp/1234/",
		"/ip4/127.0.0.1/udp/1234/quic",
		"/ip4/127.0.0.1/ipfs/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC",
		"/ip4/127.0.0.1/ipfs/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC/tcp/1234",
		"/ip4/127.0.0.1/p2p/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC",
		"/ip4/127.0.0.1/p2p/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC/tcp/1234",
		"/unix/a/b/c/d/e",
		"/unix/stdio",
		"/ip4/1.2.3.4/tcp/80/unix/a/b/c/d/e/f",
		"/ip4/127.0.0.1/ipfs/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC/tcp/1234/unix/stdio",
		"/ip4/127.0.0.1/p2p/QmcgpsyWgH8Y8ajJz1Cu72KnS5uo2Aa2LpzU7kinSupNKC/tcp/1234/unix/stdio",
		"/ip4/127.0.0.1/tcp/9090/http/p2p-webrtc-direct",
	}

	for _, a := range cases {
		if _, err := NewMultiaddr(a); err != nil {
			t.Errorf("should have succeeded: %s -- %s", a, err)
		}
	}
}

func TestEqual(t *testing.T) {
	m1 := newMultiaddr(t, "/ip4/127.0.0.1/udp/1234")
	m2 := newMultiaddr(t, "/ip4/127.0.0.1/tcp/1234")
	m3 := newMultiaddr(t, "/ip4/127.0.0.1/tcp/1234")
	m4 := newMultiaddr(t, "/ip4/127.0.0.1/tcp/1234/")

	if m1.Equal(m2) {
		t.Error("should not be equal")
	}

	if m2.Equal(m1) {
		t.Error("should not be equal")
	}

	if !m2.Equal(m3) {
		t.Error("should be equal")
	}

	if !m3.Equal(m2) {
		t.Error("should be equal")
	}

	if !m1.Equal(m1) {
		t.Error("should be equal")
	}

	if !m2.Equal(m4) {
		t.Error("should be equal")
	}

	if !m4.Equal(m3) {
		t.Error("should be equal")
	}
}

func TestStringToBytes(t *testing.T) {

	testString := func(s string, h string) {
		b1, err := hex.DecodeString(h)
		if err != nil {
			t.Error("failed to decode hex", h)
		}

		//t.Log("196", h, []byte(b1))

		b2, err := stringToBytes(s)
		if err != nil {
			t.Error("failed to convert", s, err)
		}

		if !bytes.Equal(b1, b2) {
			t.Error("failed to convert \n", s, "to\n", hex.EncodeToString(b1), "got\n", hex.EncodeToString(b2))
		}

		if err := validateBytes(b2); err != nil {
			t.Error(err, "len:", len(b2))
		}
	}

	testString("/ip4/127.0.0.1/udp/1234", "047f000001910204d2")
	testString("/ip4/127.0.0.1/tcp/4321", "047f0000010610e1")
	testString("/ip4/127.0.0.1/udp/1234/ip4/127.0.0.1/tcp/4321", "047f000001910204d2047f0000010610e1")
	testString("/onion/aaimaq4ygg2iegci:80", "bc030010c0439831b48218480050")
	testString("/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:1234", "bd03adadec040be047f9658668b11a504f3155001f231a37f54c4476c07fb4cc139ed7e30304d2")
	testString("/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA",
		"be0383038d3fc8c976a86ae4e78ba378e75ec41bc9ab1542a9cb422581987e118f5cb0c024f3639d6ad9b3aff613672f07bfbbbfc2f920ef910534ecaa6ff9c03e0fa4872a764d2fce6d4cfc5a5a9800cd95944cc9ef0241f753fe71494a175f334b35682459acadc4076428ab49b5a83a49d2ea2366b06461e4a559b0111fa750e0de0c138a94d1231ed5979572ff53922905636221994bdabc44bd0c17fef11622b16432db3f193400af53cc61aa9bfc0c4c8d874b41a6e18732f0b60f5662ef1a89c80589dd8366c90bb58bb85ead56356aba2a244950ca170abbd01094539014f84bdd383e4a10e00cee63dfc3e809506e2d9b54edbdca1bace6eaa119e68573d30533791fba830f5d80be5c051a77c09415e3b8fe3139400848be5244b8ae96bb0c4a24f819cba0488f34985eac741d3359180bd72cafa1559e4c19f54ea8cedbb6a5afde4319396eb92aab340c60a50cc2284580cb3ad09017e8d9abc60269b3d8d687680bd86ce834412273d4f2e3bf68dd3d6fe87e2426ac658cd5c77fd5c0aa000000")
	testString("/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq",
		"bf0320efbcd45d0c5dc79781ac6f20ea5055a036afb48d45a52e7d68ec7d4338919e69")

}

func TestBytesToString(t *testing.T) {

	testString := func(s1 string, h string) {
		t.Helper()
		b, err := hex.DecodeString(h)
		if err != nil {
			t.Error("failed to decode hex", h)
		}

		if err := validateBytes(b); err != nil {
			t.Error(err)
		}

		s2, err := bytesToString(b)
		if err != nil {
			t.Log("236", s1, ":", string(h), ":", s2)
			t.Error("failed to convert", b, err)
		}

		if s1 != s2 {
			t.Error("failed to convert", b, "to", s1, "got", s2)
		}
	}

	testString("/ip4/127.0.0.1/udp/1234", "047f000001910204d2")
	testString("/ip4/127.0.0.1/tcp/4321", "047f0000010610e1")
	testString("/ip4/127.0.0.1/udp/1234/ip4/127.0.0.1/tcp/4321", "047f000001910204d2047f0000010610e1")
	testString("/onion/aaimaq4ygg2iegci:80", "bc030010c0439831b48218480050")
	testString("/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:1234", "bd03adadec040be047f9658668b11a504f3155001f231a37f54c4476c07fb4cc139ed7e30304d2")
	testString("/garlic64/jT~IyXaoauTni6N4517EG8mrFUKpy0IlgZh-EY9csMAk82Odatmzr~YTZy8Hv7u~wvkg75EFNOyqb~nAPg-khyp2TS~ObUz8WlqYAM2VlEzJ7wJB91P-cUlKF18zSzVoJFmsrcQHZCirSbWoOknS6iNmsGRh5KVZsBEfp1Dg3gwTipTRIx7Vl5Vy~1OSKQVjYiGZS9q8RL0MF~7xFiKxZDLbPxk0AK9TzGGqm~wMTI2HS0Gm4Ycy8LYPVmLvGonIBYndg2bJC7WLuF6tVjVquiokSVDKFwq70BCUU5AU-EvdOD5KEOAM7mPfw-gJUG4tm1TtvcobrObqoRnmhXPTBTN5H7qDD12AvlwFGnfAlBXjuP4xOUAISL5SRLiulrsMSiT4GcugSI80mF6sdB0zWRgL1yyvoVWeTBn1TqjO27alr95DGTluuSqrNAxgpQzCKEWAyzrQkBfo2avGAmmz2NaHaAvYbOg0QSJz1PLjv2jdPW~ofiQmrGWM1cd~1cCqAAAA",
		"be0383038d3fc8c976a86ae4e78ba378e75ec41bc9ab1542a9cb422581987e118f5cb0c024f3639d6ad9b3aff613672f07bfbbbfc2f920ef910534ecaa6ff9c03e0fa4872a764d2fce6d4cfc5a5a9800cd95944cc9ef0241f753fe71494a175f334b35682459acadc4076428ab49b5a83a49d2ea2366b06461e4a559b0111fa750e0de0c138a94d1231ed5979572ff53922905636221994bdabc44bd0c17fef11622b16432db3f193400af53cc61aa9bfc0c4c8d874b41a6e18732f0b60f5662ef1a89c80589dd8366c90bb58bb85ead56356aba2a244950ca170abbd01094539014f84bdd383e4a10e00cee63dfc3e809506e2d9b54edbdca1bace6eaa119e68573d30533791fba830f5d80be5c051a77c09415e3b8fe3139400848be5244b8ae96bb0c4a24f819cba0488f34985eac741d3359180bd72cafa1559e4c19f54ea8cedbb6a5afde4319396eb92aab340c60a50cc2284580cb3ad09017e8d9abc60269b3d8d687680bd86ce834412273d4f2e3bf68dd3d6fe87e2426ac658cd5c77fd5c0aa000000")
	testString("/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq",
		"bf0320efbcd45d0c5dc79781ac6f20ea5055a036afb48d45a52e7d68ec7d4338919e69")
}

func TestBytesSplitAndJoin(t *testing.T) {

	testString := func(s string, res []string) {
		m, err := NewMultiaddr(s)
		if err != nil {
			t.Fatal("failed to convert", s, err)
		}

		split := Split(m)
		if len(split) != len(res) {
			t.Error("not enough split components", split)
			return
		}

		for i, a := range split {
			if a.String() != res[i] {
				t.Errorf("split component failed: %s != %s", a, res[i])
			}
		}

		joined := Join(split...)
		if !m.Equal(joined) {
			t.Errorf("joined components failed: %s != %s", m, joined)
		}

		for i, a := range split {
			if a.String() != res[i] {
				t.Errorf("split component failed: %s != %s", a, res[i])
			}
		}
	}

	testString("/ip4/1.2.3.4/udp/1234", []string{"/ip4/1.2.3.4", "/udp/1234"})
	testString("/ip4/1.2.3.4/tcp/1/ip4/2.3.4.5/udp/2",
		[]string{"/ip4/1.2.3.4", "/tcp/1", "/ip4/2.3.4.5", "/udp/2"})
	testString("/ip4/1.2.3.4/utp/ip4/2.3.4.5/udp/2/udt",
		[]string{"/ip4/1.2.3.4", "/utp", "/ip4/2.3.4.5", "/udp/2", "/udt"})
}

func TestProtocols(t *testing.T) {
	m, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Error("failed to construct", "/ip4/127.0.0.1/udp/1234")
	}

	ps := m.Protocols()
	if ps[0].Code != ProtocolWithName("ip4").Code {
		t.Error(ps[0], ProtocolWithName("ip4"))
		t.Error("failed to get ip4 protocol")
	}

	if ps[1].Code != ProtocolWithName("udp").Code {
		t.Error(ps[1], ProtocolWithName("udp"))
		t.Error("failed to get udp protocol")
	}

}

func TestProtocolsWithString(t *testing.T) {
	pwn := ProtocolWithName
	good := map[string][]Protocol{
		"/ip4":                    []Protocol{pwn("ip4")},
		"/ip4/tcp":                []Protocol{pwn("ip4"), pwn("tcp")},
		"ip4/tcp/udp/ip6":         []Protocol{pwn("ip4"), pwn("tcp"), pwn("udp"), pwn("ip6")},
		"////////ip4/tcp":         []Protocol{pwn("ip4"), pwn("tcp")},
		"ip4/udp/////////":        []Protocol{pwn("ip4"), pwn("udp")},
		"////////ip4/tcp////////": []Protocol{pwn("ip4"), pwn("tcp")},
	}

	for s, ps1 := range good {
		ps2, err := ProtocolsWithString(s)
		if err != nil {
			t.Errorf("ProtocolsWithString(%s) should have succeeded", s)
		}

		for i, ps1p := range ps1 {
			ps2p := ps2[i]
			if ps1p.Code != ps2p.Code {
				t.Errorf("mismatch: %s != %s, %s", ps1p.Name, ps2p.Name, s)
			}
		}
	}

	bad := []string{
		"dsijafd",                           // bogus proto
		"/ip4/tcp/fidosafoidsa",             // bogus proto
		"////////ip4/tcp/21432141/////////", // bogus proto
		"////////ip4///////tcp/////////",    // empty protos in between
	}

	for _, s := range bad {
		if _, err := ProtocolsWithString(s); err == nil {
			t.Errorf("ProtocolsWithString(%s) should have failed", s)
		}
	}

}

func TestEncapsulate(t *testing.T) {
	m, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Error(err)
	}

	m2, err := NewMultiaddr("/udp/5678")
	if err != nil {
		t.Error(err)
	}

	b := m.Encapsulate(m2)
	if s := b.String(); s != "/ip4/127.0.0.1/udp/1234/udp/5678" {
		t.Error("encapsulate /ip4/127.0.0.1/udp/1234/udp/5678 failed.", s)
	}

	m3, _ := NewMultiaddr("/udp/5678")
	c := b.Decapsulate(m3)
	if s := c.String(); s != "/ip4/127.0.0.1/udp/1234" {
		t.Error("decapsulate /udp failed.", "/ip4/127.0.0.1/udp/1234", s)
	}

	m4, _ := NewMultiaddr("/ip4/127.0.0.1")
	d := c.Decapsulate(m4)
	if d != nil {
		t.Error("decapsulate /ip4 failed: ", d)
	}
}

func assertValueForProto(t *testing.T, a Multiaddr, p int, exp string) {
	t.Logf("checking for %s in %s", ProtocolWithCode(p).Name, a)
	fv, err := a.ValueForProtocol(p)
	if err != nil {
		t.Fatal(err)
	}

	if fv != exp {
		t.Fatalf("expected %q for %d in %s, but got %q instead", exp, p, a, fv)
	}
}

func TestGetValue(t *testing.T) {
	a := newMultiaddr(t, "/ip4/127.0.0.1/utp/tcp/5555/udp/1234/utp/ipfs/QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP")
	assertValueForProto(t, a, P_IP4, "127.0.0.1")
	assertValueForProto(t, a, P_UTP, "")
	assertValueForProto(t, a, P_TCP, "5555")
	assertValueForProto(t, a, P_UDP, "1234")
	assertValueForProto(t, a, P_IPFS, "QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP")
	assertValueForProto(t, a, P_P2P, "QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP")

	_, err := a.ValueForProtocol(P_IP6)
	switch err {
	case ErrProtocolNotFound:
		break
	case nil:
		t.Fatal("expected value lookup to fail")
	default:
		t.Fatalf("expected ErrProtocolNotFound but got: %s", err)
	}

	a = newMultiaddr(t, "/ip4/0.0.0.0") // only one addr
	assertValueForProto(t, a, P_IP4, "0.0.0.0")

	a = newMultiaddr(t, "/ip4/0.0.0.0/ip4/0.0.0.0/ip4/0.0.0.0") // same sub-addr
	assertValueForProto(t, a, P_IP4, "0.0.0.0")

	a = newMultiaddr(t, "/ip4/0.0.0.0/udp/12345/utp") // ending in a no-value one.
	assertValueForProto(t, a, P_IP4, "0.0.0.0")
	assertValueForProto(t, a, P_UDP, "12345")
	assertValueForProto(t, a, P_UTP, "")

	a = newMultiaddr(t, "/ip4/0.0.0.0/unix/a/b/c/d") // ending in a path one.
	assertValueForProto(t, a, P_IP4, "0.0.0.0")
	assertValueForProto(t, a, P_UNIX, "/a/b/c/d")
}

func TestFuzzBytes(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	// Bump up these numbers if you want to stress this
	buf := make([]byte, 256)
	for i := 0; i < 2000; i++ {
		l := rand.Intn(len(buf))
		rand.Read(buf[:l])

		// just checking that it doesnt panic
		ma, err := NewMultiaddrBytes(buf[:l])
		if err == nil {
			// for any valid multiaddrs, make sure these calls don't panic
			_ = ma.String()
			ma.Protocols()
		}
	}
}

func randMaddrString() string {
	good_corpus := []string{"tcp", "ip", "udp", "ipfs", "0.0.0.0", "127.0.0.1", "12345", "QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP"}

	size := rand.Intn(256)
	parts := make([]string, 0, size)
	for i := 0; i < size; i++ {
		switch rand.Intn(5) {
		case 0, 1, 2:
			parts = append(parts, good_corpus[rand.Intn(len(good_corpus))])
		default:
			badbuf := make([]byte, rand.Intn(256))
			rand.Read(badbuf)
			parts = append(parts, string(badbuf))
		}
	}

	return "/" + strings.Join(parts, "/")
}

func TestFuzzString(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	// Bump up these numbers if you want to stress this
	for i := 0; i < 2000; i++ {

		// just checking that it doesnt panic
		ma, err := NewMultiaddr(randMaddrString())
		if err == nil {
			// for any valid multiaddrs, make sure these calls don't panic
			_ = ma.String()
			ma.Protocols()
		}
	}
}

func TestBinaryRepresentation(t *testing.T) {
	expected := []byte{0x4, 0x7f, 0x0, 0x0, 0x1, 0x91, 0x2, 0x4, 0xd2}
	ma, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(ma.Bytes(), expected) {
		t.Errorf("expected %x, got %x", expected, ma.Bytes())
	}
}

func TestRoundTrip(t *testing.T) {
	for _, s := range []string{
		"/unix/a/b/c/d",
		"/ip6/::ffff:127.0.0.1/tcp/111",
		"/ip4/127.0.0.1/tcp/123",
		"/ip4/127.0.0.1/udp/123",
		"/ip4/127.0.0.1/udp/123/ip6/::",
		"/ipfs/QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP",
		"/ipfs/QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP/unix/a/b/c",
	} {
		ma, err := NewMultiaddr(s)
		if err != nil {
			t.Errorf("error when parsing %q: %s", s, err)
			continue
		}
		if ma.String() != s {
			t.Errorf("failed to round trip %q", s)
		}
	}
}

// XXX: Change this test when we switch to /p2p by default.
func TestIPFSvP2P(t *testing.T) {
	var (
		p2pAddr  = "/p2p/QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP"
		ipfsAddr = "/ipfs/QmbHVEEepCi7rn7VL7Exxpd2Ci9NNB6ifvqwhsrbRMgQFP"
	)

	for _, s := range []string{p2pAddr, ipfsAddr} {
		ma, err := NewMultiaddr(s)
		if err != nil {
			t.Errorf("error when parsing %q: %s", s, err)
		}
		if ma.String() != ipfsAddr {
			t.Errorf("expected %q, got %q", ipfsAddr, ma.String())
		}
	}
}

func TestInvalidP2PAddr(t *testing.T) {
	badAddr := "a503221221c05877cbae039d70a5e600ea02c6f9f2942439285c9e344e26f8d280c850fad6"
	bts, err := hex.DecodeString(badAddr)
	if err != nil {
		t.Fatal(err)
	}
	ma, err := NewMultiaddrBytes(bts)
	if err == nil {
		t.Error("should have failed")
		// Check for panic
		_ = ma.String()
	}
}

func TestZone(t *testing.T) {
	ip6String := "/ip6zone/eth0/ip6/::1"
	ip6Bytes := []byte{
		0x2a, 4,
		'e', 't', 'h', '0',
		0x29,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 1,
	}

	ma, err := NewMultiaddr(ip6String)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(ma.Bytes(), ip6Bytes) {
		t.Errorf("expected %x, got %x", ip6Bytes, ma.Bytes())
	}

	ma2, err2 := NewMultiaddrBytes(ip6Bytes)
	if err2 != nil {
		t.Error(err)
	}
	if ma2.String() != ip6String {
		t.Errorf("expected %s, got %s", ip6String, ma2.String())
	}
}

func TestBinaryMarshaler(t *testing.T) {
	addr := newMultiaddr(t, "/ip4/0.0.0.0/tcp/4001")
	b, err := addr.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	var addr2 multiaddr
	if err = addr2.UnmarshalBinary(b); err != nil {
		t.Fatal(err)
	}
	if !addr.Equal(&addr2) {
		t.Error("expected equal addresses in circular marshaling test")
	}
}

func TestTextMarshaler(t *testing.T) {
	addr := newMultiaddr(t, "/ip4/0.0.0.0/tcp/4001")
	b, err := addr.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	var addr2 multiaddr
	if err = addr2.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}
	if !addr.Equal(&addr2) {
		t.Error("expected equal addresses in circular marshaling test")
	}
}

func TestJSONMarshaler(t *testing.T) {
	addr := newMultiaddr(t, "/ip4/0.0.0.0/tcp/4001")
	b, err := addr.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	var addr2 multiaddr
	if err = addr2.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}
	if !addr.Equal(&addr2) {
		t.Error("expected equal addresses in circular marshaling test")
	}
}

func TestComponentBinaryMarshaler(t *testing.T) {
	comp, err := NewComponent("ip4", "0.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := comp.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	comp2 := &Component{}
	if err = comp2.UnmarshalBinary(b); err != nil {
		t.Fatal(err)
	}
	if !comp.Equal(comp2) {
		t.Error("expected equal components in circular marshaling test")
	}
}

func TestComponentTextMarshaler(t *testing.T) {
	comp, err := NewComponent("ip4", "0.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := comp.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	comp2 := &Component{}
	if err = comp2.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}
	if !comp.Equal(comp2) {
		t.Error("expected equal components in circular marshaling test")
	}
}

func TestComponentJSONMarshaler(t *testing.T) {
	comp, err := NewComponent("ip4", "0.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := comp.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	comp2 := &Component{}
	if err = comp2.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}
	if !comp.Equal(comp2) {
		t.Error("expected equal components in circular marshaling test")
	}
}
