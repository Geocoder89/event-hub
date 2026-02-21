package utils

import (
	"strconv"
	"strings"
	"time"
)

func BuildEventsListCacheKey(limit int, city, category, tag *string, from, to *time.Time, query *string) string {
	c := ""
	if city != nil {
		c = strings.ToLower(strings.TrimSpace(*city))
	}
	cat := ""
	if category != nil {
		cat = strings.ToLower(strings.TrimSpace(*category))
	}
	tg := ""
	if tag != nil {
		tg = strings.ToLower(strings.TrimSpace(*tag))
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

	return "events:list:v3:limit=" + strconv.Itoa(limit) +
		":city=" + c +
		":category=" + cat +
		":tag=" + tg +
		":from=" + f +
		":to=" + t +
		":q=" + q
}
