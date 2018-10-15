Posts
======

We've introduced a new feature in OpenBazaar called 'posts' that allows nodes to publish social content in their directory. These posts can be used to promote a listing, store, or used in a generic way for decentralized social networking. At its core, we are increasing the tools a store can use to advertising their listings and strengthen their brand **within** the network. More broadly, we are extending the utility of OpenBazaar to be a platform for self-expression without a loss of privacy as 'table-stakes'.

## Features

There are 3 types of posts:re

1. **Post.** A generic, standalone social post.
2. **Comment.** A comment against another post, listing, or store.
3. **Repost.** The equivalent of a 'retweet', only for any type of content on the OpenBazaar network.

All posts support the following data fields:

1. **`status`.** This is a short content space where a user can post the equivalent of a 'tweet'. The maximum character limit is 280.
2. **`longForm`.** This is a longer content space than `status` ideal of Medium-style blog posts. The maximum character limit is 50,000.
3. **`images`.** A post can carry up to 30 images reference by their IPFS hashes.
4. **`channels`.** Similar to IRC, posts can be targeted for up to 30 channels. Posts can be curated based on whether posts are relevant and included in a feed for that channel.
5. **`tags`.** Each post can carry up to 30 tags, which allow for more granular discovery of a post in their store, or in a channel.
6. **`reference`.** Required when making a comment or repost, which points to the target content. While originally designed for other posts, any other valid content address on the network can be used (i.e. listing, store, image etc).

## APIs

There are 5 new APIs to support posts:

1. `GET /ob/posts`
2. `GET /ob/post`
3. `POST /ob/post`
4. `PUT /ob/post`
5. `DELETE /ob/post`

### `GET /ob/posts`

This API call retrieves a list of posts from the user's own node, or an external node. The response data will look like this:

```JSON
[
  {
    "channels": [
      "test"
    ],
    "hash": "zb2rhhXuXraeA2KVJwXmfiSVrhLh8sCQsbgzSXMPhubnnebDn",
    "images": [
      {
          "medium": "zb2rhaFhqziCWk1zo5tMRxQEUchfvJFaGG4DY1anEoR4GnYrN",
          "small": "zb2rhbDCeEiTTunugWPaRRKFCfNKUaB7aCR53nrPnMa9usZXY",
          "tiny": "zb2rhgqJDbshwAgPjs7X2h4mDm3V3BpLbp4tFGqkg1LNkg9yV"
      }
    ],
    "postType": "POST",
    "reference": "",
    "slug": "testy7",
    "status": "testy7",
    "tags": [
      "Yo"
    ],
    "timestamp": "2018-10-11T12:17:09.434846957Z"
  }
]
```

The response is an `array` of posts that the user has published. To retrieve a list of posts from an external node, the API call will need to include the target `peerId` in its path: `GET /ob/posts/{peerId}`.

### `GET /ob/post`

This API call retrieves the entire content of a post, with verifiable hashes and signatures:

```JSON
{
  "post": {
    "slug": "testy5",
    "vendorID": {
      "peerID": "QmS7QyLXGgxge2Nap3wnZr1pBdxDSJ4xu1dGVKnL4XW9YQ",
      "handle": "",
      "pubkeys": {
        "identity": "CAESINypt5aYTJ8ssxKNDusANNH9HFqKsDOC3VJT4sYhPato",
        "bitcoin": "A5EJy0XaW/S+OHn2xebf8Nb6kv54H6+lsOBlOpWXb/46"
      },
      "bitcoinSig": "MEQCIH0h0NZmJE5gWgo5soUdkggnNYSBvykWvm2DbhjW+CgHAiB309+MUBtnAkyFaaI3lEVzFUjwZRm/RU7pGFt1NT4+cA=="
    },
    "status": "testy5",
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
    "channels": [
      "test"
    ],
    "postType": "REPOST",
    "reference": "zb2rhbnwaLFrTfw8uGzZKPfS16SLt14FmEFNghrGRZUhY3g2Y",
    "timestamp": "2018-10-11T12:08:38.639363790Z"
  },
  "hash": "zb2rheDrgxGWWfiJBnFCAp7xwmK2Yt1AistCBzaYCrFB1adpN",
  "signature": "NVg62CekPbI+3j6YvaTokSi0H8Z5cbMgTW84yT+P5U6E1mYg/0vdF3c6OggW9BcSd9bhQR6wfgaWgurxmi3OBQ=="
}
```

### `POST /ob/post`

This API is used to create a post, and requires the following payload:

```JSON
{
	"status": "testy7",
	"longForm": "This is a test post dawg.",
	"postType": "POST",
	"reference": "",
	"images": [
		{
			"filename": "cat",
			"large": "zb2rhmBUB9i7UkfmeD3obJYK3FFS5K8N8QHaUanG8UWLVBHiY",
			"medium": "zb2rhaFhqziCWk1zo5tMRxQEUchfvJFaGG4DY1anEoR4GnYrN",
			"original": "zb2rhe2o6WbHqcER5VUKsMUbQrmpCC6ihg8qZ4JS9wVgKz9wm",
			"small": "zb2rhbDCeEiTTunugWPaRRKFCfNKUaB7aCR53nrPnMa9usZXY",
			"tiny": "zb2rhgqJDbshwAgPjs7X2h4mDm3V3BpLbp4tFGqkg1LNkg9yV"
		}
	],
	"tags": [
		"Yo"
	],
	"channels": [
		"test"
	]
}
```

### `PUT /ob/post`

This API is used to edit an existing post. The payload must use the `slug` of the intended post to be edited:

```JSON
{
	"status": "testy7",
	"longForm": "This is a test post son.",
	"postType": "POST",
	"reference": "",
	"images": [
		{
			"filename": "cat",
			"large": "zb2rhmBUB9i7UkfmeD3obJYK3FFS5K8N8QHaUanG8UWLVBHiY",
			"medium": "zb2rhaFhqziCWk1zo5tMRxQEUchfvJFaGG4DY1anEoR4GnYrN",
			"original": "zb2rhe2o6WbHqcER5VUKsMUbQrmpCC6ihg8qZ4JS9wVgKz9wm",
			"small": "zb2rhbDCeEiTTunugWPaRRKFCfNKUaB7aCR53nrPnMa9usZXY",
			"tiny": "zb2rhgqJDbshwAgPjs7X2h4mDm3V3BpLbp4tFGqkg1LNkg9yV"
		}
	],
	"tags": [
		"Yo"
	],
	"channels": [
		"test"
	]
}
```

### `DELETE /ob/post`

This API is used to delete a post. The `slug` of the target post must be included in the URL path: `DELETE /ob/post/{slug}`.

## Future work

There are two primary areas of work that need improvments:

1. Feed
2. Reactions

### Feed

The 'feed' is simply a list of posts based on the nodes you follow, or posts published to a certain channel or with a tag. The feed will not require a user to fetch a list of posts manually from each node that they follow. Currently, third parties can construct a social feed of posts based on a list of `peerIds`, a channel, or tag. We aim to explore technologies such as IPFS pubsub and [OrbitDB](https://github.com/orbitdb/orbit-db) in further detail to remove this requirement and allow a more decentralized construction of feeds.

### Reactions

'Reactions' are social reactions: like, love, smile etc. While this data could exist as its own type of post, this solution isn't scalable and would require frequent republishing of the root IPNS directory, and undermining the ability of peers to seed the latest version of the store. A decentralised solution is required to handle rapidly dynamic data; again, OrbitDB seems like a good candidate to investigate.
