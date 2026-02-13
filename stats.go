package tempuscache

type Stats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
}
