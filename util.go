package main

import (
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgtype"
	_ "github.com/mattn/go-sqlite3"
)

type feature struct {
	Username string `json:"username"`
	Feat     string `json:"features"`
}

type ytvod struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Start     string `json:"starttime"`
	End       string `json:"endtime"`
	Thumbnail string `json:"thumbnail"`
}

type rumblevod struct {
	PublicID  string `json:"public_id"`
	EmbedID   string `json:"embed_id"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	Thumbnail string `json:"thumbnail"`
	Start     string `json:"starttime"`
	End       string `json:"endtime"`
}

type embed struct {
	Link     string `json:"link"`
	Platform string `json:"platform"`
	Channel  string `json:"channel"`
	Title    string `json:"title"`
	Count    int    `json:"count"`
}

type lastembed struct {
	Link      string `json:"link"`
	Platform  string `json:"platform"`
	Channel   string `json:"channel"`
	Title     string `json:"title"`
	Timestamp int    `json:"timestamp"`
}

type phrase struct {
	Time     time.Time `json:"time"`
	Username string    `json:"username"`
	Phrase   string    `json:"phrase"`
	Duration string    `json:"duration"`
	Type     string    `json:"type"`
}

type lwodUrl struct {
	ID string
}

type lwodTwitch struct {
	Start   string `json:"starttime"`
	End     string `json:"endtime"`
	Game    string `json:"game"`
	Subject string `json:"subject"`
	Topic   string `json:"topic"`
}

type lwodYT struct {
	Time    int    `json:"time"`
	Game    string `json:"game"`
	Subject string `json:"subject"`
	Topic   string `json:"topic"`
}

type lwod struct {
	Twitch  *string `json:"twitch"`
	YouTube *string `json:"youtube"`
	Start   string  `json:"starttime"`
	End     string  `json:"endtime"`
	Game    string  `json:"game"`
	Subject string  `json:"subject"`
	Topic   string  `json:"topic"`
}

type logLineString struct {
	Time     string `json:"time"`
	Username string `json:"username"`
	Features string `json:"features"`
	Message  string `json:"message"`
}

type logGroup struct {
	Time  int64
	Lines pgtype.JSONArray
}

type logLine struct {
	Time     time.Time `json:"time"`
	Username string    `json:"username"`
	Features string    `json:"features"`
	Message  string    `json:"message"`
}

type nuke struct {
	Time     time.Time `json:"time"`
	Type     string    `json:"type"`
	Duration string    `json:"duration"`
	Word     string    `json:"word"`
	Victims  string    `json:"victims"`
}

type msgCount struct {
	Count int `json:"count"`
}

func MinMax(array []time.Time) (time.Time, time.Time) {
	var max time.Time = array[0]
	var min time.Time = array[0]
	for _, value := range array {
		if int(max.Unix()) < int(value.Unix()) {
			max = value
		}
		if int(min.Unix()) > int(value.Unix()) {
			min = value
		}
	}
	return min, max
}

func indexOf(element time.Time, data []time.Time) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}
	return -1
}

func indexOfUnnuke(data []string, word string) int {
	for k, v := range data {
		if word == v {
			return k
		}
	}
	return -1
}

func removeNukeByIndex(data []nuke, s int) []nuke {
	return append(data[:s], data[s+1:]...)
}
