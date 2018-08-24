package api

import (
	"fmt"
)

const notFoundJSON = `{"success": false,"reason": "Not Found"}`

const jsonUnexpectedEOF = `{"success": false,"reason": "unexpected EOF"}`

// AlreadyExistsUsePUTJSON generates an error message expected when
// attempted to recreate a resource
func AlreadyExistsUsePUTJSON(resource string) string {
	return fmt.Sprintf(`{
        "success": false,
        "reason": "%s already exists. Use PUT."
    }`, resource)
}

func NotFoundJSON(resource string) string {
	return fmt.Sprintf(`{
        "success": false,
        "reason": "%s not found."
    }`, resource)
}

//
// Settings
//

const settingsJSON = `{
    "version": "",
    "paymentDataInQR": true,
    "showNotifications": true,
    "showNsfw": true,
    "shippingAddresses": [{
        "name": "Seymour Butts",
        "company": "Globex Corporation",
        "addressLineOne": "31 Spooner Street",
        "addressLineTwo": "Apt. 124",
        "city": "Quahog",
        "state": "RI",
        "country": "UNITED_STATES",
        "postalCode": "",
        "addressNotes": "Leave package at back door"
    }],
    "localCurrency": "USD",
    "country": "UNITED_STATES",
    "termsAndConditions": "By purchasing this item you agree to the following...",
    "refundPolicy": "All sales are final.",
    "blockedNodes": ["QmecpJrN9RJ7smyYByQdZUy5mF6aapgCfKLKRmDtycv9aG", "QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr", "QmPDLS7TV9Q3gtxRXQVqrm2RpEtz1Mq6u2YGeuEJWCqu6B"],
    "storeModerators": ["QmNedYJ6WmLhacAL2ozxb4k33Gxd9wmKB7HyoxZCwXid1e", "QmQdi7EaJUmuRUtSaCPkijw5cptFfNcX2EPvMyQwR117Y2"],
	"mispaymentBuffer": 1,
    "smtpSettings": {
        "notifications": true,
        "serverAddress": "smtp.urbanart.com:465",
        "username": "urbanart",
        "password": "letmein",
        "senderEmail": "notifications@urbanart.com",
        "recipientEmail": "Dave@gmail.com"
    }
}`

const settingsUpdateJSON = `{
	"version": "",
    "paymentDataInQR": false,
    "showNotifications": true,
    "showNsfw": false,
    "shippingAddresses": [{
        "name": "I.C. Wiener",
        "company": "Globex Corporation",
        "addressLineOne": "31 Spooner Street",
        "addressLineTwo": "Apt. 124",
        "city": "Quahog",
        "state": "RI",
        "country": "UNITED_STATES",
        "postalCode": "",
        "addressNotes": "Leave package at front door"
    }],
    "localCurrency": "BTC",
    "country": "UNITED_STATES",
    "termsAndConditions": "By purchasing this item you agree to the following...",
    "refundPolicy": "All sales are final.",
    "blockedNodes": ["QmecpJrN9RJ7smyYByQdZUy5mF6aapgCfKLKRmDtycv9aG", "QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr", "QmPDLS7TV9Q3gtxRXQVqrm2RpEtz1Mq6u2YGeuEJWCqu6B"],
    "storeModerators": ["QmNedYJ6WmLhacAL2ozxb4k33Gxd9wmKB7HyoxZCwXid1e", "QmQdi7EaJUmuRUtSaCPkijw5cptFfNcX2EPvMyQwR117Y2"],
	"mispaymentBuffer": 1,
    "smtpSettings": {
        "notifications": true,
        "serverAddress": "smtp.urbanart.com:465",
        "username": "urbanart",
        "password": "letmein",
        "senderEmail": "notifications@urbanart.com",
        "recipientEmail": "Dave@gmail.com"
    }
}`

const settingsPatchJSON = `{
    "paymentDataInQR": true,
    "showNotifications": true,
    "showNsfw": true,
    "shippingAddresses": [{
        "name": "Craig Wright"
    }]
}`

const settingsPatchedJSON = `{
	"version": "",
    "paymentDataInQR": true,
    "showNotifications": true,
    "showNsfw": true,
    "shippingAddresses": [{
        "name": "Craig Wright",
        "company": "",
        "addressLineOne": "",
        "addressLineTwo": "",
        "city": "",
        "state": "",
        "country": "",
        "postalCode": "",
        "addressNotes": ""
    }],
    "localCurrency": "BTC",
    "country": "UNITED_STATES",
    "termsAndConditions": "By purchasing this item you agree to the following...",
    "refundPolicy": "All sales are final.",
    "blockedNodes": ["QmecpJrN9RJ7smyYByQdZUy5mF6aapgCfKLKRmDtycv9aG", "QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr", "QmPDLS7TV9Q3gtxRXQVqrm2RpEtz1Mq6u2YGeuEJWCqu6B"],
    "storeModerators": ["QmNedYJ6WmLhacAL2ozxb4k33Gxd9wmKB7HyoxZCwXid1e", "QmQdi7EaJUmuRUtSaCPkijw5cptFfNcX2EPvMyQwR117Y2"],
	"mispaymentBuffer": 1,
    "smtpSettings": {
        "notifications": true,
        "serverAddress": "smtp.urbanart.com:465",
        "username": "urbanart",
        "password": "letmein",
        "senderEmail": "notifications@urbanart.com",
        "recipientEmail": "Dave@gmail.com"
    }
}`

const settingsMalformedJSON = `{
    /"paymentDataInQR": false,
}`

const settingsMalformedJSONResponse = `{
    "success": false,
    "reason": "invalid character '/' looking for beginning of object key string"
}`

const settingsAlreadyExistsJSON = `{
    "success": false,
    "reason": "Settings is already set. Use PUT."
}`

//
// Profile
//

const profileJSON = `{
    "peerID": "QmSpuEe2XZy5DNYQHgL5uhe6DiaJWDiDkH2q1yjhoFd9PP",
    "handle": "satoshi",
    "name": "Satoshi Nakamoto",
    "location": "Japan",
    "about": "Bitcoins Creator",
    "shortDescription": "I make money",
    "contactInfo": {
	    "website": "bitcoin.org",
	    "email": "satoshi@gmx.com",
	    "phoneNumber": "5551234567"
    },
    "nsfw": true,
    "vendor": true,
    "moderator": false,
    "colors": {
	    "primary": "#000000",
	    "secondary": "#FFD700",
	    "text": "#ffffff",
	    "highlight": "#123ABC",
	    "highlightText": "#DEAD00"
    },
    "stats": {
	    "followerCount": 1,
	    "followingCount": 2,
	    "listingCount": 3,
	    "ratingCount": 21000000,
	    "averageRating": 1
    },
    "bitcoinPubkey": "0314e6def3bd71e2806d87ae06ec88ca175701b34ae308f81c16266f69ddc98053"
}`

const profileUpdateJSON = `{
    "handle": "satoshi",
    "name": "Craig Wright",
    "location": "Australia"
}`

const profileUpdatedJSON = `{
    "peerID": "QmSpuEe2XZy5DNYQHgL5uhe6DiaJWDiDkH2q1yjhoFd9PP",
    "handle": "satoshi",
    "name": "Craig Wright",
    "location": "Australia",
    "about": "",
    "shortDescription": "",
    "nsfw": false,
    "vendor": false,
    "moderator": false,
    "bitcoinPubkey": "0314e6def3bd71e2806d87ae06ec88ca175701b34ae308f81c16266f69ddc98053"
}`

//
// Images
//

const avatarValidJSON = `{
	"avatar": "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"
}`

const avatarValidJSONResponse = `{
    "large": "zb2rhbEB183zRmazfsY7ytQWfyPEQUuLy3Ryso4h8jkwSxQY6",
    "medium": "zb2rhhf3ZxHrUyb43sQ2ivQTJvWUPgTyF5T2h4fM8xLxmtHuU",
    "original": "zb2rhmfTQq3yixcKnEy5q68ujAayJLP3KsxjAR5gMocFcRQy5",
    "small": "zb2rhY2W79RCbZDM87t9pVL4CHicYjQxmVih3q7u7QRDc5xqP",
    "tiny": "zb2rhbrsSgjXymYSGqH6pvma72g6WRmx69sbCsc5ANCbVMAJs"
}`

const avatarUnexpectedEOFJSON = `{
	"avatar": "/9j/4AAQSkZJRgABAQAAAQABAA"
}`

const avatarUnexpectedEOFJSONResponse = `{
    "success": false,
    "reason": "unexpected EOF"
}`

var avatarInvalidTQJSON = avatarValidJSON[:100] + `0` + avatarValidJSON[100:]

const avatarInvalidTQJSONResponse = `{
    "success": false,
    "reason": "invalid JPEG format: bad Tq value"
}`

const imageValidJSON = `[{
	"filename": "blue_tshirt.jpg",
	"image": "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"
}]`

const imageValidJSONResponse = `[{
        "filename": "blue_tshirt.jpg",
        "hashes": {
            "large": "zb2rhkProdDprVM9jGYwyN1sGTrTVQXebAup6oLLpQfcC9WsK",
            "medium": "zb2rhZqKFguaaEodgYKHUUmvDVWeFpF2eWMNgvhxrdoKYjUKw",
            "original": "zb2rhcuzHEovhfy4u2mvGkghj5FyEHZKu1fkzMB8t7MXo7fru",
            "small": "zb2rhgKxh4gLKrqoVYmE3gzJve61PVwHP277rSQh4jyL4ftrK",
            "tiny": "zb2rhnmhLobuoHHpbXoz3YE2aaptRvxMg4DCUY62MVr56ooUL"
        }
}]`

const headerValidJSON = `{
	"header": "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"
}`

const headerValidJSONResponse = `{
    "large": "zb2rhmowivBcCAR9XQFm8yTYiescwZjzYqJzyiqsVuQwEpYFr",
    "medium": "zb2rhhwb8anfu1GmJrD1yZcqp6AmxHA3h4yfNr1RdV8gqci5y",
    "original": "zb2rhmfTQq3yixcKnEy5q68ujAayJLP3KsxjAR5gMocFcRQy5",
    "small": "zb2rhXFSGMLsi5sSmVb5rrTx7E9biwycYzYVpAAbAkk7pypB5",
    "tiny": "zb2rhbQwzGdgYeiqy213dqrFJkDruvid3EVVFUgkWn6samyCA"
}`

//
// Inventory
//

const inventoryUpdateJSON = `[{
	"slug": "ron_swanson_tshirt",
	"variant": 0,
	"quantity": 17
}]`

//
// Moderation
//

const moderatorValidJSON = `{
	"description": "Long time OpenBazaar moderator located in Chicago",
	"termsAndConditions": "Will moderate anything and everything",
	"languages": ["English", "Spanish"],
	"fee": {
		"feeType": "FIXED_PLUS_PERCENTAGE",
		"fixedFee": {
			"currencyCode": "USD",
			"amount": 300
		},
		"percentage": 5
	}
}`

//
// Wallet
//

const walletMneumonicJSONResponse = `{"mnemonic": "correct horse battery staple"}`

const walletAddressJSONResponse = `{"address": "moLsBry5Dk8AN3QT3i1oxZdwD12MYRfTL5"}`

const walletBalanceJSONResponse = `{"confirmed": 0, "unconfirmed": 0, "height": 0}`

//
// Spending
//

const spendJSON = `{
	"address": "1HYhu8e2wv19LZ2umXoo1pMiwzy2rL32UQ",
	"amount": 1700000,
	"feeLevel": "NORMAL"
}`

const insuffientFundsJSON = `{
	"success": false,
	"reason": "ERROR_INSUFFICIENT_FUNDS"
}`

//
// Posts
//

const postJSON = `{
	"slug": "test1",
	"vendorID": {
			"peerID": "QmRxFnmPSJdRN9vckxBW7W9bcz7grJNoqbk1dRvuNtPSUD",
			"handle": "",
			"pubkeys": {
					"identity": "CAESIPdtZDIN2wNQU0BLXgVJjPBr75qTwXU5DDn6kQkZPsd7",
					"bitcoin": "AleyRDzqIrUSnORZvR/qcICSRWSkooFkODE3qzT5k1fY"
			},
			"bitcoinSig": "MEUCIQD08Qj1Ahu/8HgJPV/jq5Qxr2Nc5ixQxqAYo4+GZMFBCgIgfeWgWQ+ZXcT1pNiUSyldD1UaAaihtDXgaGYZZYD9boQ="
	},
	"title": "test1",
	"longForm": "This is a test post dawg.",
	"images": [
			{
					"filename": "cat",
					"original": "zb2rhe2o6WbHqcER5VUKsMUbQrmpCC6ihg8qZ4JS9wVgKz9wm",
					"large": "zb2rhmBUB9i7UkfmeD3obJYK3FFS5K8N8QHaUanG8UWLVBHiY",
					"medium": "zb2rhaFhqziCWk1zo5tMRxQEUchfvJFaGG4DY1anEoR4GnYrN",
					"small": "zb2rhbDCeEiTTunugWPaRRKFCfNKUaB7aCR53nrPnMa9usZXY",
					"tiny": "zb2rhgqJDbshwAgPjs7X2h4mDm3V3BpLbp4tFGqkg1LNkg9yV"
			}
	],
	"tags": [
			"Yo"
	],
	"timestamp": "2017-11-02T04:15:07.972887695Z"
}`

const postJSONResponse = `{"slug": "test1"}`

const postUpdateJSON = `{
	"slug": "test1",
	"vendorID": {
			"peerID": "QmRxFnmPSJdRN9vckxBW7W9bcz7grJNoqbk1dRvuNtPSUD",
			"handle": "",
			"pubkeys": {
					"identity": "CAESIPdtZDIN2wNQU0BLXgVJjPBr75qTwXU5DDn6kQkZPsd7",
					"bitcoin": "AleyRDzqIrUSnORZvR/qcICSRWSkooFkODE3qzT5k1fY"
			},
			"bitcoinSig": "MEUCIQD08Qj1Ahu/8HgJPV/jq5Qxr2Nc5ixQxqAYo4+GZMFBCgIgfeWgWQ+ZXcT1pNiUSyldD1UaAaihtDXgaGYZZYD9boQ="
	},
	"title": "test1",
	"longForm": "This is a test post dawgs and cats.",
	"images": [
			{
					"filename": "cat",
					"original": "zb2rhe2o6WbHqcER5VUKsMUbQrmpCC6ihg8qZ4JS9wVgKz9wm",
					"large": "zb2rhmBUB9i7UkfmeD3obJYK3FFS5K8N8QHaUanG8UWLVBHiY",
					"medium": "zb2rhaFhqziCWk1zo5tMRxQEUchfvJFaGG4DY1anEoR4GnYrN",
					"small": "zb2rhbDCeEiTTunugWPaRRKFCfNKUaB7aCR53nrPnMa9usZXY",
					"tiny": "zb2rhgqJDbshwAgPjs7X2h4mDm3V3BpLbp4tFGqkg1LNkg9yV"
			}
	],
	"tags": [
			"Yo"
	],
	"timestamp": "2017-11-02T04:16:09.281618842Z"
}`
