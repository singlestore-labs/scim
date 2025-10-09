package util

type Trace interface {
	Logf(string, ...interface{})
}
