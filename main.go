package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var trustedProxyIP string
var pgUser string
var pgPass string
var pgHost string
var pgPort string
var pgName string
var redisHost string
var redisPort string
var redisPass string

var featdb *sql.DB
var lwoddb *sql.DB
var ytvoddb *sql.DB
var embeddb *sql.DB
var pg *pgxpool.Pool
var rdb *redis.Client

var nukeRegex *regexp.Regexp
var mutelinksRegex *regexp.Regexp

func init() {
	log.SetHandler(text.New((os.Stderr)))
}

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
	ID      string `json:"id"`
	Start   string `json:"starttime"`
	End     string `json:"endtime"`
	Game    string `json:"game"`
	Subject string `json:"subject"`
	Topic   string `json:"topic"`
}

type logLineString struct {
	Time     string `json:"time"`
	Username string `json:"username"`
	Features string `json:"features"`
	Message  string `json:"message"`
}

type logGroup struct {
	Time  int64
	Lines pgtype.JSONBArray
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

func getScript(c *fiber.Ctx) error {
	if c.Params("dev") == "" {
		scriptVersion, err := rdb.Get(context.Background(), "SCRIPT_VERSION").Result()
		if err != nil {
			log.Errorf("[%s] %s %s - redis query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		scriptLink, err := rdb.Get(context.Background(), "SCRIPT_LINK").Result()
		if err != nil {
			log.Errorf("[%s] %s %s - redis query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		return c.JSON(&fiber.Map{
			"version": scriptVersion,
			"link":    scriptLink,
		})
	} else {
		devScriptVersion, err := rdb.Get(context.Background(), "DEV_SCRIPT_VERSION").Result()
		if err != nil {
			log.Errorf("[%s] %s %s - redis query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		devScriptLink, err := rdb.Get(context.Background(), "DEV_SCRIPT_LINK").Result()
		if err != nil {
			log.Errorf("[%s] %s %s - redis query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		return c.JSON(&fiber.Map{
			"version": devScriptVersion,
			"link":    devScriptLink,
		})
	}

}

func getFeatures(c *fiber.Ctx) error {
	feats := []feature{}
	var featsFormatted map[string]string = make(map[string]string)

	rows, err := featdb.Query("SELECT * from dggfeat")
	if err != nil {
		log.Errorf("[%s] %s %s - featdb query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := feature{}
		err := rows.Scan(&p.Username, &p.Feat)
		if err != nil {
			log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		feats = append(feats, p)
	}

	for _, feat := range feats {
		featsFormatted[feat.Username] = feat.Feat
	}

	return c.JSON(featsFormatted)
}

func getYTvods(c *fiber.Ctx) error {
	ytvods := []ytvod{}

	rows, err := ytvoddb.Query("SELECT vodid, title, starttime, endtime, thumbnail from ytvods ORDER BY datetime(starttime) DESC LIMIT 45")
	if err != nil {
		log.Errorf("[%s] %s %s - ytvoddb query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := ytvod{}
		err := rows.Scan(&p.ID, &p.Title, &p.Start, &p.End, &p.Thumbnail)
		if err != nil {
			log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		ytvods = append(ytvods, p)
	}

	return c.JSON(ytvods)
}

func getEmbeds(c *fiber.Ctx) error {
	timeString := c.Query("t")
	embeds := []embed{}
	lastembeds := []lastembed{}

	if c.Params("last") == "" {
		if timeString != "" {
			timeInt, err := strconv.Atoi(timeString)
			if err != nil {
				log.Errorf("[%s] %s %s - String to int conversion error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				return c.Status(500).SendString("The time parameter is invalid")
			}
			if timeInt >= 5 && timeInt <= 60 {
				rows, err := embeddb.Query("select link, platform, channel, title, count(link) as freq from embeds where timest >= strftime('%s', 'now') - $1 group by link order by freq desc limit 5", timeInt*60)
				if err != nil {
					log.Errorf("[%s] %s %s - embeddb query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
					return c.SendStatus(500)
				}
				defer rows.Close()

				for rows.Next() {
					p := embed{}
					err := rows.Scan(&p.Link, &p.Platform, &p.Channel, &p.Title, &p.Count)
					if err != nil {
						log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
						continue
					}
					embeds = append(embeds, p)
				}
			} else {
				return c.Status(400).SendString("Time needs to be between 5 and 60 minutes")
			}
		} else {
			return c.Status(400).SendString("The time parameter has not been provided")
		}
		return c.JSON(embeds)
	} else {
		rows, err := embeddb.Query("select timest, link, platform, channel, title from embeds order by timest desc limit 5")
		if err != nil {
			log.Errorf("[%s] %s %s - embeddb query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lastembed{}
			err := rows.Scan(&p.Timestamp, &p.Link, &p.Platform, &p.Channel, &p.Title)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			lastembeds = append(lastembeds, p)
		}

		return c.JSON(lastembeds)
	}
}

func getPhrases(c *fiber.Ctx) error {
	countString := c.Query("count")
	phrases := []phrase{}

	if countString != "" {
		count, err := strconv.Atoi(countString)
		if err != nil {
			log.Errorf("[%s] %s %s - String to int conversion error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.Status(500).SendString("The count parameter is invalid")
		}
		rows, err := pg.Query(context.Background(), "select * from phrases order by time desc limit $1", count)
		if err != nil {
			log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := phrase{}
			err := rows.Scan(&p.Time, &p.Username, &p.Phrase, &p.Duration, &p.Type)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			phrases = append(phrases, p)
		}
	} else {
		rows, err := pg.Query(context.Background(), "select * from phrases order by time desc")
		if err != nil {
			log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := phrase{}
			err := rows.Scan(&p.Time, &p.Username, &p.Phrase, &p.Duration, &p.Type)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			phrases = append(phrases, p)
		}
	}

	return c.JSON(phrases)
}

func getLWOD(c *fiber.Ctx) error {
	vodid := c.Query("id")
	vidid := c.Query("v")

	twitchEntries := []lwodTwitch{}
	youtubeEntries := []lwodYT{}
	allEntries := []lwod{}

	if vodid != "" {
		rows, err := lwoddb.Query("SELECT starttime, endtime, game, subject, topic from lwod WHERE vodid=$1", vodid)
		if err != nil {
			log.Errorf("[%s] %s %s - lwoddb query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lwodTwitch{}
			err := rows.Scan(&p.Start, &p.End, &p.Game, &p.Subject, &p.Topic)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			twitchEntries = append(twitchEntries, p)
		}

		return c.JSON(twitchEntries)
	} else if vidid != "" {
		rows, err := lwoddb.Query("SELECT yttime, game, subject, topic from lwod WHERE vidid=$1 ORDER by yttime", vidid)
		if err != nil {
			log.Errorf("[%s] %s %s - lwoddb query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lwodYT{}
			err := rows.Scan(&p.Time, &p.Game, &p.Subject, &p.Topic)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			youtubeEntries = append(youtubeEntries, p)
		}

		return c.JSON(youtubeEntries)
	} else {
		rows, err := lwoddb.Query("SELECT vodid, starttime, endtime, game, subject, topic from lwod")
		if err != nil {
			log.Errorf("[%s] %s %s - lwoddb query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lwod{}
			err := rows.Scan(&p.ID, &p.Start, &p.End, &p.Game, &p.Subject, &p.Topic)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			allEntries = append(allEntries, p)
		}

		return c.JSON(allEntries)
	}
}

func getLogs(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	logsRaw := []logGroup{}
	var logs map[int64][]pgtype.JSONB = make(map[int64][]pgtype.JSONB)

	if from != "" && to != "" {
		rows, err := pg.Query(context.Background(), "SELECT extract(epoch from date_trunc('second', time)), array_agg(json_build_object('username', username, 'features', features, 'message', message)) FROM logs WHERE time >= $1 AND time < $2 GROUP BY date_trunc('second', time) ORDER BY date_trunc('second', time)", from, to)
		if err != nil {
			log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := logGroup{}
			err := rows.Scan(&p.Time, &p.Lines)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			logsRaw = append(logsRaw, p)
		}

		for _, group := range logsRaw {
			logs[group.Time] = group.Lines.Elements
		}
	}

	return c.JSON(logs)
}

func getRawLogs(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	logs := []logLineString{}

	if from != "" && to != "" {
		rows, err := pg.Query(context.Background(), "SELECT to_char(time, 'YYYY-MM-DD\"T\"HH24:MI:SS.MSZ'), username, features, message FROM logs WHERE time >= $1 AND time < $2 ORDER BY time", from, to)
		if err != nil {
			log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := logLineString{}
			err := rows.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
			if err != nil {
				log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			logs = append(logs, p)
		}
	}

	return c.JSON(logs)
}

func getNukes(c *fiber.Ctx) error {
	logs := []logLine{}
	countRaw := []logLine{}
	countStamps := []time.Time{}
	data := []nuke{}

	rows1, err := pg.Query(context.Background(), "select * from nukes where message ~* '^(!nuke|!meganuke|!aegis|!aegissingle|!an|!unnuke|!as)' and features ~ '(moderator|admin)' and time >= NOW() - INTERVAL '5 minutes' order by time desc FETCH FIRST 10 ROWS ONLY")
	if err != nil {
		log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows1.Close()

	for rows1.Next() {
		p := logLine{}
		err := rows1.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
		if err != nil {
			fmt.Println(err)
			continue
		}
		logs = append(logs, p)
	}

	rows2, err := pg.Query(context.Background(), "select * from nukes where message ~ 'Dropping the NUKE on' and features ~ '(bot)' and time >= NOW() - INTERVAL '5 minutes' order by time desc FETCH FIRST 10 ROWS ONLY")
	if err != nil {
		log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
	}
	defer rows2.Close()

	for rows2.Next() {
		p := logLine{}
		err := rows2.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
		if err != nil {
			log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		countRaw = append(countRaw, p)
	}

	for _, line := range countRaw {
		countStamps = append(countStamps, line.Time)
	}

	unnukeSlice := []string{}
	nukeType := ""

	for _, line := range logs {
		if line.Message[:1] == "!" {
			msgSplit := strings.SplitN(line.Message, " ", 2)
			switch len(msgSplit) {
			case 0:
				continue
			case 1:
				nukeType = msgSplit[0][1:]
				switch nukeType {
				case "aegis":
					return c.JSON(data)
				case "aegissingle", "an", "unnuke", "as":
					if len(msgSplit) > 1 {
						if len(msgSplit[1]) > 0 {
							unnukeSlice = append(unnukeSlice, msgSplit[1])
						}
					}
					continue
				default:
					continue
				}
			default:
				nukeType = msgSplit[0][1:]
				switch nukeType {
				case "aegis":
					return c.JSON(data)
				case "aegissingle", "an", "unnuke", "as":
					if len(msgSplit) > 1 {
						if len(msgSplit[1]) > 0 {
							unnukeSlice = append(unnukeSlice, msgSplit[1])
						}
					}
					if len(unnukeSlice) > 0 {
						for _, element := range data {
							index := indexOfUnnuke(unnukeSlice, element.Word)
							if index != -1 {
								removeNukeByIndex(data, index)
							}
						}
					}
				default:
					nukedCount := ""
					if len(countStamps) > 0 {
						minTime, _ := MinMax(countStamps)
						if int(minTime.Unix()) < int(line.Time.Unix())+1 {
							buf := countRaw[indexOf(minTime, countStamps)].Message
							nukedCount = buf[21 : len(buf)-8]
						}
					}
					theRest := msgSplit[1]
					match := nukeRegex.FindAllStringSubmatch(theRest, -1)
					for i := range match {
						if len(match[i][2]) != 0 {
							unnukeIndex := indexOfUnnuke(unnukeSlice, match[i][2])
							if len(match[i][1]) != 0 {
								if unnukeIndex == -1 {
									if nukedCount != "" {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: match[i][1],
											Word:     match[i][2],
											Victims:  nukedCount,
										})
									} else {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: match[i][1],
											Word:     match[i][2],
										})
									}
								}
							} else {
								if unnukeIndex == -1 {
									if nukedCount != "" {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: "10m",
											Word:     match[i][2],
											Victims:  nukedCount,
										})
									} else {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: "10m",
											Word:     match[i][2],
										})
									}
								}
							}
						} else {
							unnukeIndex := indexOfUnnuke(unnukeSlice, match[i][3])
							if len(match[i][1]) != 0 {
								if unnukeIndex == -1 {
									if nukedCount != "" {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: match[i][1],
											Word:     match[i][3],
											Victims:  nukedCount,
										})
									} else {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: match[i][1],
											Word:     match[i][3],
										})
									}
								}
							} else {
								if unnukeIndex == -1 {
									if nukedCount != "" {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: "10m",
											Word:     match[i][3],
											Victims:  nukedCount,
										})
									} else {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: "10m",
											Word:     match[i][3],
										})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return c.JSON(data)
}

func getMutelinks(c *fiber.Ctx) error {
	logs := []logLine{}

	rows, err := pg.Query(context.Background(), "select * from mutelinks where message ~* '^(!mutelinks|!mutelink|!linkmute|!linksmute)' and features ~ '(moderator|admin)' order by time desc FETCH FIRST 1 ROWS ONLY")
	if err != nil {
		log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := logLine{}
		err := rows.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
		if err != nil {
			log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		logs = append(logs, p)
	}

	for _, line := range logs {
		if line.Message[:1] == "!" {
			msgSplit := strings.SplitN(line.Message, " ", 2)
			switch len(msgSplit) {
			case 0:
				continue
			case 1:
				continue
			default:
				theRest := msgSplit[1]
				match := mutelinksRegex.FindAllStringSubmatch(theRest, -1)
				for i := range match {
					if len(match[i][2]) != 0 {
						return c.JSON([]fiber.Map{{
							"time":     line.Time,
							"status":   match[i][1],
							"duration": match[i][2],
							"user":     line.Username,
						},
						})
					} else {
						return c.JSON([]fiber.Map{{
							"time":     line.Time,
							"status":   match[i][1],
							"duration": "10m",
							"user":     line.Username,
						},
						})
					}
				}
			}
		}
	}

	return c.Status(404).SendString("Don't have data for mutelinks")
}

func getMsgCount(c *fiber.Ctx) error {
	username := c.Query("u")
	count := msgCount{}

	rows, err := pg.Query(context.Background(), "select count(*) from logs where username ~* $1 and time >= current_date::timestamp and time < current_date::timestamp + interval '1 day'", username)
	if err != nil {
		log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := msgCount{}
		err := rows.Scan(&p.Count)
		if err != nil {
			log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		count = p
	}

	return c.JSON(count)
}

func getLastLWODSheet(c *fiber.Ctx) error {
	lastLWODSheet := lwodUrl{}

	row := lwoddb.QueryRow("SELECT sheetId from lwodUrl ORDER BY datetime(date) DESC LIMIT 1")

	err := row.Scan(&lastLWODSheet.ID)
	if err != nil {
		log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path(), err)
		return c.SendStatus(500)
	}

	return c.Redirect(fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s", lastLWODSheet.ID))
}

func loadDotEnv() {
	log.Infof("[%s] Loading environment variables", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	godotenv.Load()
	trustedProxyIP = os.Getenv("TRUSTED_PROXY")
	if trustedProxyIP == "" {
		log.Fatalf("[%s] Please set the TRUSTED_PROXY environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	pgUser = os.Getenv("POSTGRES_USER")
	if pgUser == "" {
		log.Fatalf("[%s] Please set the POSTGRES_USER environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	pgPass = os.Getenv("POSTGRES_PASSWORD")
	if pgPass == "" {
		log.Fatalf("[%s] Please set the POSTGRES_PASSWORD environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	pgHost = os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		log.Fatalf("[%s] Please set the POSTGRES_HOST environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	pgPort = os.Getenv("POSTGRES_PORT")
	if pgPort == "" {
		log.Fatalf("[%s] Please set the POSTGRES_PORT environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	pgName = os.Getenv("POSTGRES_DB")
	if pgName == "" {
		log.Fatalf("[%s] Please set the POSTGRES_DB environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	redisHost = os.Getenv("REDIS_HOST")
	if redisHost == "" {
		log.Fatalf("[%s] Please set the REDIS_HOST environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	redisPort = os.Getenv("REDIS_PORT")
	if redisPort == "" {
		log.Fatalf("[%s] Please set the REDIS_PORT environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}
	redisPass = os.Getenv("REDIS_PASSWORD")

	loc, err := time.LoadLocation("UTC")
	if err != nil {
		log.Fatalf("[%s] %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}
	time.Local = loc

	log.Infof("[%s] Environment variables loaded successfully", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
}

func loadDatabases() {
	log.Infof("[%s] Connecting to databases", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	dbpath := filepath.Join(".", "db")
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		log.Fatalf("[%s] Error creating a db directory: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}

	pgUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", pgUser, pgPass, pgHost, pgPort, pgName)

	dbpath = filepath.Join(".", "db", "featdb.db")
	featdb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("[%s] Error opening featdb: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}

	dbpath = filepath.Join(".", "db", "lwoddb.db")
	lwoddb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("[%s] Error opening lwoddb: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}

	dbpath = filepath.Join(".", "db", "ytvoddb.db")
	ytvoddb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("[%s] Error opening ytvoddb: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}

	dbpath = filepath.Join(".", "db", "embeddb.db")
	embeddb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("[%s] Error opening embeddb: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}

	pg, err = pgxpool.Connect(context.Background(), pgUrl)
	if err != nil {
		log.Fatalf("[%s] Error connecting to Postgres DB: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: redisPass,
		DB:       0,
	})

	log.Infof("[%s] Connected to databases successfully", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
}

func compileRegexp() {
	log.Infof("[%s] Compiling regexp", time.Now().Format("2006-01-02 15:04:05.000000 MST"))

	nukeRegex = regexp.MustCompile(`(\d+[HMDSWwhmds])?\s?(?:\/(.*)\/)?(.*)`)
	mutelinksRegex = regexp.MustCompile(`(?P<state>on|off|all)(?:(?:\s+)(?P<time>\d+[HMDSWwhmds]))?`)

	log.Infof("[%s] Regexp compiled successfully", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
}

func main() {
	loadDotEnv()
	loadDatabases()
	compileRegexp()

	api := fiber.New(fiber.Config{
		ProxyHeader:             "X-Forwarded-For",
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{trustedProxyIP},
		Immutable:               true,
	})

	api.Use(cors.New())
	api.Use(limiter.New(limiter.Config{
		Max: 60,
	}))
	api.Use(logger.New(logger.Config{
		Format:     "[${time}] ${ip} - ${status} ${method} ${path}${query:} - ${latency}\n",
		TimeFormat: "2006-01-02 15:04:05.000000 MST",
	}))

	api.Get(os.Getenv("API_PREFIX")+"/script/:dev?", getScript)
	api.Get(os.Getenv("API_PREFIX")+"/features", getFeatures)
	api.Get(os.Getenv("API_PREFIX")+"/ytvods", getYTvods)
	api.Get(os.Getenv("API_PREFIX")+"/embeds/:last?", getEmbeds)
	api.Get(os.Getenv("API_PREFIX")+"/phrases", getPhrases)
	api.Get(os.Getenv("API_PREFIX")+"/lwod", getLWOD)
	api.Get(os.Getenv("API_PREFIX")+"/logs", getLogs)
	api.Get(os.Getenv("API_PREFIX")+"/rawlogs", getRawLogs)
	api.Get(os.Getenv("API_PREFIX")+"/nukes", getNukes)
	api.Get(os.Getenv("API_PREFIX")+"/mutelinks", getMutelinks)
	api.Get(os.Getenv("API_PREFIX")+"/msgcount", getMsgCount)
	api.Get(os.Getenv("API_PREFIX")+"/lastlwod", getLastLWODSheet)

	if os.Getenv("PORT") == "" {
		log.Fatalf("[%s] Please set the PORT environment variable and restart the server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}

	api.Listen(":" + os.Getenv("PORT"))
}
