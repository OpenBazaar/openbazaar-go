package factory

import "github.com/OpenBazaar/multiwallet/model"

func NewTransaction() model.Transaction {
	return model.Transaction{
		Txid:     "1be612e4f2b79af279e0b307337924072b819b3aca09fcb20370dd9492b83428",
		Version:  2,
		Locktime: 512378,
		Inputs: []model.Input{
			{
				Txid:       "6d892f04fc097f430d58ab06229c9b6344a130fc1842da5b990e857daed42194",
				Vout:       1,
				Sequence:   1,
				ValueIface: "0.04294455",
				ScriptSig: model.Script{
					Hex: "4830450221008665481674067564ef562cfd8d1ca8f1506133fb26a2319e4b8dfba3cedfd5de022038f27121c44e6c64b93b94d72620e11b9de35fd864730175db9176ca98f1ec610121022023e49335a0dddb864ff673468a6cc04e282571b1227933fcf3ff9babbcc662",
				},
				Addr:     "1C74Gbij8Q5h61W58aSKGvXK4rk82T2A3y",
				Satoshis: 4294455,
			},
		},
		Outputs: []model.Output{
			{
				ScriptPubKey: model.OutScript{
					Script: model.Script{
						Hex: "76a914ff3f7d402fbd6d116ba4a02af9784f3ae9b7108a88ac",
					},
					Type:      "pay-to-pubkey-hash",
					Addresses: []string{"1QGdNEDjWnghrjfTBCTDAPZZ3ffoKvGc9B"},
				},
				ValueIface: "0.01398175",
			},
			{
				ScriptPubKey: model.OutScript{
					Script: model.Script{
						Hex: "a9148a62462d08a977fa89226a56fca7eb01b6fef67c87",
					},
					Type:      "pay-to-script-hashh",
					Addresses: []string{"3EJiuDqsHuAtFqiLGWKVyCfvqoGpWVCCRs"},
				},
				ValueIface: "0.02717080",
			},
		},
		Time:          1520449061,
		BlockHash:     "0000000000000000003f1fb88ac3dab0e607e87def0e9031f7bea02cb464a04f",
		BlockHeight:   512476,
		Confirmations: 1,
	}
}
