package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/vyneer/vyneer-api/logger"
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
var rumbledb *sql.DB
var omnimirrordb *sql.DB
var embeddb *sql.DB
var pg *pgxpool.Pool
var rdb *redis.Client

var nukeRegex *regexp.Regexp
var regexCheck *regexp.Regexp
var mutelinksRegex *regexp.Regexp

func init() {
	log.SetHandler(log.New((os.Stderr)))
}

func getScript(c *fiber.Ctx) error {
	if c.Params("dev") == "" {
		scriptVersion, err := rdb.Get(context.Background(), "SCRIPT_VERSION").Result()
		if err != nil {
			log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		scriptLink, err := rdb.Get(context.Background(), "SCRIPT_LINK").Result()
		if err != nil {
			log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		return c.JSON(&fiber.Map{
			"version": scriptVersion,
			"link":    scriptLink,
		})
	} else {
		devScriptVersion, err := rdb.Get(context.Background(), "DEV_SCRIPT_VERSION").Result()
		if err != nil {
			log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		devScriptLink, err := rdb.Get(context.Background(), "DEV_SCRIPT_LINK").Result()
		if err != nil {
			log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
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
		log.Errorf("%s %s - featdb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := feature{}
		err := rows.Scan(&p.Username, &p.Feat)
		if err != nil {
			log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
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
		log.Errorf("%s %s - ytvoddb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := ytvod{}
		err := rows.Scan(&p.ID, &p.Title, &p.Start, &p.End, &p.Thumbnail)
		if err != nil {
			log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		ytvods = append(ytvods, p)
	}

	return c.JSON(ytvods)
}

func getRumbleVods(c *fiber.Ctx) error {
	rumblevods := []rumblevod{}

	rows, err := rumbledb.Query("SELECT public_id, embed_id, title, link, thumbnail, start_time, end_time from rumble ORDER BY datetime(start_time) DESC LIMIT 45")
	if err != nil {
		log.Errorf("%s %s - rumbledb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := rumblevod{}
		err := rows.Scan(&p.PublicID, &p.EmbedID, &p.Title, &p.Link, &p.Thumbnail, &p.Start, &p.End)
		if err != nil {
			log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		rumblevods = append(rumblevods, p)
	}

	return c.JSON(rumblevods)
}

func getOmnimirrorVods(c *fiber.Ctx) error {
	rumblevods := []rumblevod{}

	rows, err := omnimirrordb.Query("SELECT public_id, embed_id, title, link, thumbnail, start_time, end_time from rumble ORDER BY datetime(start_time) DESC LIMIT 45")
	if err != nil {
		log.Errorf("%s %s - omnimirrordb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := rumblevod{}
		err := rows.Scan(&p.PublicID, &p.EmbedID, &p.Title, &p.Link, &p.Thumbnail, &p.Start, &p.End)
		if err != nil {
			log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			continue
		}
		rumblevods = append(rumblevods, p)
	}

	return c.JSON(rumblevods)
}

func getEmbeds(c *fiber.Ctx) error {
	timeString := c.Query("t")
	embeds := []embed{}
	lastembeds := []lastembed{}

	if c.Params("last") == "" {
		if timeString != "" {
			timeInt, err := strconv.Atoi(timeString)
			if err != nil {
				log.Errorf("%s %s - String to int conversion error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				return c.Status(500).SendString("The time parameter is invalid")
			}
			if timeInt >= 5 && timeInt <= 60 {
				rows, err := embeddb.Query("select link, platform, channel, title, count(link) as freq from embeds where timest >= strftime('%s', 'now') - $1 group by link order by freq desc limit 5", timeInt*60)
				if err != nil {
					log.Errorf("%s %s - embeddb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
					return c.SendStatus(500)
				}
				defer rows.Close()

				for rows.Next() {
					p := embed{}
					err := rows.Scan(&p.Link, &p.Platform, &p.Channel, &p.Title, &p.Count)
					if err != nil {
						log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
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
			log.Errorf("%s %s - embeddb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lastembed{}
			err := rows.Scan(&p.Timestamp, &p.Link, &p.Platform, &p.Channel, &p.Title)
			if err != nil {
				log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			lastembeds = append(lastembeds, p)
		}

		return c.JSON(lastembeds)
	}
}

func getPhrases(c *fiber.Ctx) error {
	countString := c.Query("count")
	phrases, err := phrases(countString)
	if err != nil {
		switch {
		case errors.Is(err, strconv.ErrSyntax):
			log.Errorf("%s %s - String to int conversion error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.Status(500).SendString("The count parameter is invalid")
		default:
			log.Errorf("%s %s - Phrases error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			c.SendStatus(500)
		}
		log.Errorf("%s %s - String to int conversion error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.Status(500).SendString("The count parameter is invalid")
	}

	if c.Query("ts") == "1" {
		if phraseStamp >= phraseRemovalStamp {
			return c.JSON(fiber.Map{
				"updatedAt": phraseStamp,
				"data":      phrases,
			})
		} else {
			return c.JSON(fiber.Map{
				"updatedAt": phraseRemovalStamp,
				"data":      phrases,
			})
		}
	} else {
		return c.JSON(phrases)
	}
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
			log.Errorf("%s %s - lwoddb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lwodTwitch{}
			err := rows.Scan(&p.Start, &p.End, &p.Game, &p.Subject, &p.Topic)
			if err != nil {
				log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			twitchEntries = append(twitchEntries, p)
		}

		return c.JSON(twitchEntries)
	} else if vidid != "" {
		rows, err := lwoddb.Query("SELECT yttime, game, subject, topic from lwod WHERE vidid=$1 ORDER by yttime", vidid)
		if err != nil {
			log.Errorf("%s %s - lwoddb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lwodYT{}
			err := rows.Scan(&p.Time, &p.Game, &p.Subject, &p.Topic)
			if err != nil {
				log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			youtubeEntries = append(youtubeEntries, p)
		}

		return c.JSON(youtubeEntries)
	} else {
		rows, err := lwoddb.Query("SELECT vodid, vidid, starttime, endtime, game, subject, topic from lwod")
		if err != nil {
			log.Errorf("%s %s - lwoddb query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := lwod{}
			err := rows.Scan(&p.Twitch, &p.YouTube, &p.Start, &p.End, &p.Game, &p.Subject, &p.Topic)
			if err != nil {
				log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
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
	var logs map[int64][]pgtype.JSON = make(map[int64][]pgtype.JSON)

	if from != "" && to != "" {
		rows, err := pg.Query(context.Background(), "SELECT extract(epoch from date_trunc('second', time)), array_agg(json_build_object('username', username, 'features', features, 'message', message)) FROM logs WHERE time >= $1 AND time < $2 GROUP BY date_trunc('second', time) ORDER BY date_trunc('second', time)", from, to)
		if err != nil {
			log.Errorf("%s %s - Postgres query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := logGroup{}
			err := rows.Scan(&p.Time, &p.Lines)
			if err != nil {
				log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
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
			log.Errorf("%s %s - Postgres query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return c.SendStatus(500)
		}
		defer rows.Close()

		for rows.Next() {
			p := logLineString{}
			err := rows.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
			if err != nil {
				log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			logs = append(logs, p)
		}
	}

	return c.JSON(logs)
}

func getNukes(c *fiber.Ctx) error {
	data, err := nukes()
	if err != nil {
		log.Errorf("%s %s - Nukes error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}

	if c.Query("ts") == "1" {
		return c.JSON(fiber.Map{
			"updatedAt": nukeStamp,
			"data":      data,
		})
	} else {
		return c.JSON(data)
	}
}

func getMutelinks(c *fiber.Ctx) error {
	mutelinks, err := mutelinks()
	if err != nil {
		log.Errorf("%s %s - Mutelinks error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	if mutelinks != nil {
		if c.Query("ts") == "1" {
			return c.JSON(fiber.Map{
				"updatedAt": mutelinksStamp,
				"data":      mutelinks,
			})
		} else {
			return c.JSON(mutelinks)
		}
	} else {
		return c.Status(404).SendString("Don't have data for mutelinks")
	}
}

func getMsgCount(c *fiber.Ctx) error {
	username := c.Query("u")
	count := msgCount{}

	rows, err := pg.Query(context.Background(), "select count(*) from logs where username ~* $1 and time >= current_date::timestamp and time < current_date::timestamp + interval '1 day'", username)
	if err != nil {
		log.Errorf("%s %s - Postgres query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	defer rows.Close()

	for rows.Next() {
		p := msgCount{}
		err := rows.Scan(&p.Count)
		if err != nil {
			log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
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
		log.Errorf("%s %s - Query scan error: %s", c.Method(), c.Path(), err)
		return c.SendStatus(500)
	}

	return c.Redirect(fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s", lastLWODSheet.ID))
}

func getProviders(c *fiber.Ctx) error {
	embedsProvider, err := rdb.Get(context.Background(), "SCRIPT_EMBEDS_PROVIDER").Result()
	if err != nil {
		log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	phrasesProvider, err := rdb.Get(context.Background(), "SCRIPT_PHRASES_PROVIDER").Result()
	if err != nil {
		log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	nukesProvider, err := rdb.Get(context.Background(), "SCRIPT_NUKES_PROVIDER").Result()
	if err != nil {
		log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}
	linksProvider, err := rdb.Get(context.Background(), "SCRIPT_LINKS_PROVIDER").Result()
	if err != nil {
		log.Errorf("%s %s - redis query error: %s", c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return c.SendStatus(500)
	}

	return c.JSON(&fiber.Map{
		"embeds":  embedsProvider,
		"phrases": phrasesProvider,
		"nukes":   nukesProvider,
		"links":   linksProvider,
	})
}

func loadDotEnv() {
	log.Infof("Loading environment variables")
	godotenv.Load()
	trustedProxyIP = os.Getenv("TRUSTED_PROXY")
	if trustedProxyIP == "" {
		log.Fatalf("Please set the TRUSTED_PROXY environment variable and restart the server")
	}
	pgUser = os.Getenv("POSTGRES_USER")
	if pgUser == "" {
		log.Fatalf("Please set the POSTGRES_USER environment variable and restart the server")
	}
	pgPass = os.Getenv("POSTGRES_PASSWORD")
	if pgPass == "" {
		log.Fatalf("Please set the POSTGRES_PASSWORD environment variable and restart the server")
	}
	pgHost = os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		log.Fatalf("Please set the POSTGRES_HOST environment variable and restart the server")
	}
	pgPort = os.Getenv("POSTGRES_PORT")
	if pgPort == "" {
		log.Fatalf("Please set the POSTGRES_PORT environment variable and restart the server")
	}
	pgName = os.Getenv("POSTGRES_DB")
	if pgName == "" {
		log.Fatalf("Please set the POSTGRES_DB environment variable and restart the server")
	}
	redisHost = os.Getenv("REDIS_HOST")
	if redisHost == "" {
		log.Fatalf("Please set the REDIS_HOST environment variable and restart the server")
	}
	redisPort = os.Getenv("REDIS_PORT")
	if redisPort == "" {
		log.Fatalf("Please set the REDIS_PORT environment variable and restart the server")
	}
	redisPass = os.Getenv("REDIS_PASSWORD")

	loc, err := time.LoadLocation("UTC")
	if err != nil {
		log.Fatalf("%s", err)
	}
	time.Local = loc

	log.Infof("Environment variables loaded successfully")
}

func loadDatabases() {
	log.Infof("Connecting to databases")
	dbpath := filepath.Join(".", "db")
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		log.Fatalf("Error creating a db directory: %s", err)
	}

	pgUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", pgUser, pgPass, pgHost, pgPort, pgName)

	dbpath = filepath.Join(".", "db", "featdb.db")
	featdb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error opening featdb: %s", err)
	}

	dbpath = filepath.Join(".", "db", "lwoddb.db")
	lwoddb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error opening lwoddb: %s", err)
	}

	dbpath = filepath.Join(".", "db", "ytvoddb.db")
	ytvoddb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error opening ytvoddb: %s", err)
	}

	dbpath = filepath.Join(".", "db", "rumble.sqlite")
	rumbledb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error opening rumbledb: %s", err)
	}

	dbpath = filepath.Join(".", "db", "omnimirror.sqlite")
	omnimirrordb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error opening omnimirrordb: %s", err)
	}

	dbpath = filepath.Join(".", "db", "embeddb.db")
	embeddb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error opening embeddb: %s", err)
	}

	pg, err = pgxpool.Connect(context.Background(), pgUrl)
	if err != nil {
		log.Fatalf("Error connecting to Postgres DB: %s", err)
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: redisPass,
		DB:       0,
	})

	log.Infof("Connected to databases successfully")
}

func compileRegexp() {
	log.Infof("Compiling regexp")

	nukeRegex = regexp.MustCompile(`(\d+[HMDSWwhmds])?\s?(?:\/(.*)\/)?(.*)`)
	regexCheck = regexp.MustCompile(`/(?:\/(.*)\/)?(.*)/`)
	mutelinksRegex = regexp.MustCompile(`(?P<state>on|off|all)(?:(?:\s+)(?P<time>\d+[HMDSWwhmds]))?`)

	log.Infof("Regexp compiled successfully")
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
	api.Use(fiberLogger.New(fiberLogger.Config{
		Format:     "[${time}] ${ip} - ${status} ${method} ${path}${query:} - ${latency}\n",
		TimeFormat: "2006-01-02 15:04:05.000000 MST",
	}))

	api.Get(os.Getenv("API_PREFIX")+"/script/:dev?", getScript)
	api.Get(os.Getenv("API_PREFIX")+"/features", getFeatures)
	api.Get(os.Getenv("API_PREFIX")+"/ytvods", getYTvods)
	api.Get(os.Getenv("API_PREFIX")+"/rumblevods", getRumbleVods)
	api.Get(os.Getenv("API_PREFIX")+"/omnimirror", getOmnimirrorVods)
	api.Get(os.Getenv("API_PREFIX")+"/embeds/:last?", getEmbeds)
	api.Get(os.Getenv("API_PREFIX")+"/phrases", getPhrases)
	api.Get(os.Getenv("API_PREFIX")+"/lwod", getLWOD)
	api.Get(os.Getenv("API_PREFIX")+"/logs", getLogs)
	api.Get(os.Getenv("API_PREFIX")+"/rawlogs", getRawLogs)
	api.Get(os.Getenv("API_PREFIX")+"/nukes", getNukes)
	api.Get(os.Getenv("API_PREFIX")+"/mutelinks", getMutelinks)
	api.Get(os.Getenv("API_PREFIX")+"/msgcount", getMsgCount)
	api.Get(os.Getenv("API_PREFIX")+"/lastlwod", getLastLWODSheet)
	api.Get(os.Getenv("API_PREFIX")+"/nmptimestamps", checkStamps)
	api.Get(os.Getenv("API_PREFIX")+"/providers", getProviders)

	if os.Getenv("PORT") == "" {
		log.Fatalf("Please set the PORT environment variable and restart the server")
	}

	go func() {
		for {
			doubleCheckStamps()
			time.Sleep(time.Second * 15)
		}
	}()
	go gRPCServer()

	api.Listen(":" + os.Getenv("PORT"))
}
