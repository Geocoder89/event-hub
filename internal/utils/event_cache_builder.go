package utils

import (
	"strconv"
	"strings"
	"time"
)

func BuildEventsListCacheKey(limit int, city *string, from, to *time.Time, query *string) string {
	c := ""
	if city != nil {
		c = strings.ToLower(strings.TrimSpace(*city))
	}
	q := ""
	if query != nil {
		q = strings.ToLower(strings.TrimSpace(*query))
	}
	f := ""
	if from != nil {
		f = from.UTC().Format(time.RFC3339Nano)
	}
	t := ""
	if to != nil {
		t = to.UTC().Format(time.RFC3339Nano)
	}

	return "events:list:v2:limit=" + strconv.Itoa(limit) +
		":city=" + c +
		":from=" + f +
		":to=" + t +
		":q=" + q
}
