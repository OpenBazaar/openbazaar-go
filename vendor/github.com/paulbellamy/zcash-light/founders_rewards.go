package zcash

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
)

var foundersRewardAddress = map[string][]string{
	chaincfg.MainNetParams.Name: []string{
		"t3Vz22vK5z2LcKEdg16Yv4FFneEL1zg9ojd", /* main-index: 0*/
		"t3cL9AucCajm3HXDhb5jBnJK2vapVoXsop3", /* main-index: 1*/
		"t3fqvkzrrNaMcamkQMwAyHRjfDdM2xQvDTR", /* main-index: 2*/
		"t3TgZ9ZT2CTSK44AnUPi6qeNaHa2eC7pUyF", /* main-index: 3*/
		"t3SpkcPQPfuRYHsP5vz3Pv86PgKo5m9KVmx", /* main-index: 4*/
		"t3Xt4oQMRPagwbpQqkgAViQgtST4VoSWR6S", /* main-index: 5*/
		"t3ayBkZ4w6kKXynwoHZFUSSgXRKtogTXNgb", /* main-index: 6*/
		"t3adJBQuaa21u7NxbR8YMzp3km3TbSZ4MGB", /* main-index: 7*/
		"t3K4aLYagSSBySdrfAGGeUd5H9z5Qvz88t2", /* main-index: 8*/
		"t3RYnsc5nhEvKiva3ZPhfRSk7eyh1CrA6Rk", /* main-index: 9*/
		"t3Ut4KUq2ZSMTPNE67pBU5LqYCi2q36KpXQ", /* main-index: 10*/
		"t3ZnCNAvgu6CSyHm1vWtrx3aiN98dSAGpnD", /* main-index: 11*/
		"t3fB9cB3eSYim64BS9xfwAHQUKLgQQroBDG", /* main-index: 12*/
		"t3cwZfKNNj2vXMAHBQeewm6pXhKFdhk18kD", /* main-index: 13*/
		"t3YcoujXfspWy7rbNUsGKxFEWZqNstGpeG4", /* main-index: 14*/
		"t3bLvCLigc6rbNrUTS5NwkgyVrZcZumTRa4", /* main-index: 15*/
		"t3VvHWa7r3oy67YtU4LZKGCWa2J6eGHvShi", /* main-index: 16*/
		"t3eF9X6X2dSo7MCvTjfZEzwWrVzquxRLNeY", /* main-index: 17*/
		"t3esCNwwmcyc8i9qQfyTbYhTqmYXZ9AwK3X", /* main-index: 18*/
		"t3M4jN7hYE2e27yLsuQPPjuVek81WV3VbBj", /* main-index: 19*/
		"t3gGWxdC67CYNoBbPjNvrrWLAWxPqZLxrVY", /* main-index: 20*/
		"t3LTWeoxeWPbmdkUD3NWBquk4WkazhFBmvU", /* main-index: 21*/
		"t3P5KKX97gXYFSaSjJPiruQEX84yF5z3Tjq", /* main-index: 22*/
		"t3f3T3nCWsEpzmD35VK62JgQfFig74dV8C9", /* main-index: 23*/
		"t3Rqonuzz7afkF7156ZA4vi4iimRSEn41hj", /* main-index: 24*/
		"t3fJZ5jYsyxDtvNrWBeoMbvJaQCj4JJgbgX", /* main-index: 25*/
		"t3Pnbg7XjP7FGPBUuz75H65aczphHgkpoJW", /* main-index: 26*/
		"t3WeKQDxCijL5X7rwFem1MTL9ZwVJkUFhpF", /* main-index: 27*/
		"t3Y9FNi26J7UtAUC4moaETLbMo8KS1Be6ME", /* main-index: 28*/
		"t3aNRLLsL2y8xcjPheZZwFy3Pcv7CsTwBec", /* main-index: 29*/
		"t3gQDEavk5VzAAHK8TrQu2BWDLxEiF1unBm", /* main-index: 30*/
		"t3Rbykhx1TUFrgXrmBYrAJe2STxRKFL7G9r", /* main-index: 31*/
		"t3aaW4aTdP7a8d1VTE1Bod2yhbeggHgMajR", /* main-index: 32*/
		"t3YEiAa6uEjXwFL2v5ztU1fn3yKgzMQqNyo", /* main-index: 33*/
		"t3g1yUUwt2PbmDvMDevTCPWUcbDatL2iQGP", /* main-index: 34*/
		"t3dPWnep6YqGPuY1CecgbeZrY9iUwH8Yd4z", /* main-index: 35*/
		"t3QRZXHDPh2hwU46iQs2776kRuuWfwFp4dV", /* main-index: 36*/
		"t3enhACRxi1ZD7e8ePomVGKn7wp7N9fFJ3r", /* main-index: 37*/
		"t3PkLgT71TnF112nSwBToXsD77yNbx2gJJY", /* main-index: 38*/
		"t3LQtHUDoe7ZhhvddRv4vnaoNAhCr2f4oFN", /* main-index: 39*/
		"t3fNcdBUbycvbCtsD2n9q3LuxG7jVPvFB8L", /* main-index: 40*/
		"t3dKojUU2EMjs28nHV84TvkVEUDu1M1FaEx", /* main-index: 41*/
		"t3aKH6NiWN1ofGd8c19rZiqgYpkJ3n679ME", /* main-index: 42*/
		"t3MEXDF9Wsi63KwpPuQdD6by32Mw2bNTbEa", /* main-index: 43*/
		"t3WDhPfik343yNmPTqtkZAoQZeqA83K7Y3f", /* main-index: 44*/
		"t3PSn5TbMMAEw7Eu36DYctFezRzpX1hzf3M", /* main-index: 45*/
		"t3R3Y5vnBLrEn8L6wFjPjBLnxSUQsKnmFpv", /* main-index: 46*/
		"t3Pcm737EsVkGTbhsu2NekKtJeG92mvYyoN", /* main-index: 47*/
		// "t3PZ9PPcLzgL57XRSG5ND4WNBC9UTFb8DXv", /* main-index: 48*/
		// "t3L1WgcyQ95vtpSgjHfgANHyVYvffJZ9iGb", /* main-index: 49*/
		// "t3JtoXqsv3FuS7SznYCd5pZJGU9di15mdd7", /* main-index: 50*/
		// "t3hLJHrHs3ytDgExxr1mD8DYSrk1TowGV25", /* main-index: 51*/
		// "t3fmYHU2DnVaQgPhDs6TMFVmyC3qbWEWgXN", /* main-index: 52*/
		// "t3T4WmAp6nrLkJ24iPpGeCe1fSWTPv47ASG", /* main-index: 53*/
		// "t3fP6GrDM4QVwdjFhmCxGNbe7jXXXSDQ5dv", /* main-index: 54*/
	},
	chaincfg.RegressionNetParams.Name: []string{
		"t2FwcEhFdNXuFMv1tcYwaBJtYVtMj8b1uTg",
	},
	chaincfg.TestNet3Params.Name: []string{
		"t2UNzUUx8mWBCRYPRezvA363EYXyEpHokyi",
		"t2N9PH9Wk9xjqYg9iin1Ua3aekJqfAtE543",
		"t2NGQjYMQhFndDHguvUw4wZdNdsssA6K7x2",
		"t2ENg7hHVqqs9JwU5cgjvSbxnT2a9USNfhy",
		"t2BkYdVCHzvTJJUTx4yZB8qeegD8QsPx8bo",
		"t2J8q1xH1EuigJ52MfExyyjYtN3VgvshKDf",
		"t2Crq9mydTm37kZokC68HzT6yez3t2FBnFj",
		"t2EaMPUiQ1kthqcP5UEkF42CAFKJqXCkXC9",
		"t2F9dtQc63JDDyrhnfpzvVYTJcr57MkqA12",
		"t2LPirmnfYSZc481GgZBa6xUGcoovfytBnC",
		"t26xfxoSw2UV9Pe5o3C8V4YybQD4SESfxtp",
		"t2D3k4fNdErd66YxtvXEdft9xuLoKD7CcVo",
		"t2DWYBkxKNivdmsMiivNJzutaQGqmoRjRnL",
		"t2C3kFF9iQRxfc4B9zgbWo4dQLLqzqjpuGQ",
		"t2MnT5tzu9HSKcppRyUNwoTp8MUueuSGNaB",
		"t2AREsWdoW1F8EQYsScsjkgqobmgrkKeUkK",
		"t2Vf4wKcJ3ZFtLj4jezUUKkwYR92BLHn5UT",
		"t2K3fdViH6R5tRuXLphKyoYXyZhyWGghDNY",
		"t2VEn3KiKyHSGyzd3nDw6ESWtaCQHwuv9WC",
		"t2F8XouqdNMq6zzEvxQXHV1TjwZRHwRg8gC",
		"t2BS7Mrbaef3fA4xrmkvDisFVXVrRBnZ6Qj",
		"t2FuSwoLCdBVPwdZuYoHrEzxAb9qy4qjbnL",
		"t2SX3U8NtrT6gz5Db1AtQCSGjrpptr8JC6h",
		"t2V51gZNSoJ5kRL74bf9YTtbZuv8Fcqx2FH",
		"t2FyTsLjjdm4jeVwir4xzj7FAkUidbr1b4R",
		"t2EYbGLekmpqHyn8UBF6kqpahrYm7D6N1Le",
		"t2NQTrStZHtJECNFT3dUBLYA9AErxPCmkka",
		"t2GSWZZJzoesYxfPTWXkFn5UaxjiYxGBU2a",
		"t2RpffkzyLRevGM3w9aWdqMX6bd8uuAK3vn",
		"t2JzjoQqnuXtTGSN7k7yk5keURBGvYofh1d",
		"t2AEefc72ieTnsXKmgK2bZNckiwvZe3oPNL",
		"t2NNs3ZGZFsNj2wvmVd8BSwSfvETgiLrD8J",
		"t2ECCQPVcxUCSSQopdNquguEPE14HsVfcUn",
		"t2JabDUkG8TaqVKYfqDJ3rqkVdHKp6hwXvG",
		"t2FGzW5Zdc8Cy98ZKmRygsVGi6oKcmYir9n",
		"t2DUD8a21FtEFn42oVLp5NGbogY13uyjy9t",
		"t2UjVSd3zheHPgAkuX8WQW2CiC9xHQ8EvWp",
		"t2TBUAhELyHUn8i6SXYsXz5Lmy7kDzA1uT5",
		"t2Tz3uCyhP6eizUWDc3bGH7XUC9GQsEyQNc",
		"t2NysJSZtLwMLWEJ6MH3BsxRh6h27mNcsSy",
		"t2KXJVVyyrjVxxSeazbY9ksGyft4qsXUNm9",
		"t2J9YYtH31cveiLZzjaE4AcuwVho6qjTNzp",
		"t2QgvW4sP9zaGpPMH1GRzy7cpydmuRfB4AZ",
		"t2NDTJP9MosKpyFPHJmfjc5pGCvAU58XGa4",
		"t29pHDBWq7qN4EjwSEHg8wEqYe9pkmVrtRP",
		"t2Ez9KM8VJLuArcxuEkNRAkhNvidKkzXcjJ",
		"t2D5y7J5fpXajLbGrMBQkFg2mFN8fo3n8cX",
		"t2UV2wr1PTaUiybpkV3FdSdGxUJeZdZztyt",
	},
}

var founderRewardScripts = func() map[string][][]byte {
	// TODO: Doing this every time we boot is inefficient, if it is an issue, we
	// should pre-generate these (or use a better way to validate coinbase
	// transactions).
	scripts := map[string][][]byte{}
	for networkName, addresses := range foundersRewardAddress {
		params, ok := netParams[networkName]
		if !ok {
			panic(fmt.Sprintf("could not find network params for founder rewards on network: %v", networkName))
		}
		scripts[networkName] = make([][]byte, len(addresses))
		for i, addrStr := range addresses {
			addr, err := DecodeAddress(addrStr, &params)
			if err != nil {
				panic(err)
			}
			scripts[networkName][i], err = PayToAddrScript(addr)
			if err != nil {
				panic(err)
			}
		}
	}
	return scripts
}()

var netParams = map[string]chaincfg.Params{
	chaincfg.MainNetParams.Name:       chaincfg.MainNetParams,
	chaincfg.RegressionNetParams.Name: chaincfg.RegressionNetParams,
	chaincfg.TestNet3Params.Name:      chaincfg.TestNet3Params,
}
