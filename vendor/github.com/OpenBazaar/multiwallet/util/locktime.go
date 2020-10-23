package util

import "errors"

func LockTimeFromRedeemScript(redeemScript []byte) (uint32, error) {
	if len(redeemScript) < 113 {
		return 0, errors.New("Redeem script invalid length")
	}
	if redeemScript[106] != 103 {
		return 0, errors.New("Invalid redeem script")
	}
	if redeemScript[107] == 0 {
		return 0, nil
	}
	if 81 <= redeemScript[107] && redeemScript[107] <= 96 {
		return uint32((redeemScript[107] - 81) + 1), nil
	}
	var v []byte
	op := redeemScript[107]
	if 1 <= op && op <= 75 {
		for i := 0; i < int(op); i++ {
			v = append(v, []byte{redeemScript[108+i]}...)
		}
	} else {
		return 0, errors.New("Too many bytes pushed for sequence")
	}
	var result int64
	for i, val := range v {
		result |= int64(val) << uint8(8*i)
	}

	return uint32(result), nil
}
