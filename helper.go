package main

import (
	"crypto/rand"
	"io"
	"time"
)

// Random returns a variable number of bytes of random data.
func Random(n int) ([]byte, error) {
	k := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, k[:])
	if err != nil {
		return nil, err
	}

	return k, nil
}

// GetFutureTime extends a base time to a future time according to the
// duration param criteria
func GetFutureTime(date *time.Time, days time.Duration, hours time.Duration, minutes time.Duration, seconds time.Duration) *time.Time {
	duration := ((time.Hour * 24) * days) + (time.Hour * hours) +
		(time.Minute * minutes) + (time.Second * seconds)
	futureTime := date.Add(duration)
	return &futureTime
}
