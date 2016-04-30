package repo

type Datastore interface {
	Followers() Followers
}

type Followers interface {
	Put(follower string) error
	Get(startIndex uint, numToReturn uint) []string
	Delete(follower string) error
}
