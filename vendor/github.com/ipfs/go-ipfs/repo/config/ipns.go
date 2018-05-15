package config

type Ipns struct {
	RepublishPeriod string
	RecordLifetime  string

	ResolveCacheSize int
	QuerySize        int

	UsePersistentCache bool

	BackUpAPI string
}
