package repo

type Datastore interface {
	Followers() Followers
	Close()
}

type Followers interface {
	// Put a B58 encoded follower ID to the database
	Put(follower string) error

	// Get followers from the database.
	// The offset and limit arguments can be used to for lazy loading.
	Get(offset int, limit int) ([]string, error)

	// Delete a follower from the databse.
	Delete(follower string) error

	// Return the number of followers in the database.
	Count() int
}
