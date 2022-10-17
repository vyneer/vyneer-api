package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/gofiber/fiber/v2"
	_ "github.com/mattn/go-sqlite3"
)

func nukes() ([]nuke, error) {
	logs := []logLine{}
	countRaw := []logLine{}
	countStamps := []time.Time{}
	data := []nuke{}

	rows1, err := pg.Query(context.Background(), "select * from nukes where message ~* '^(!nuke|!meganuke|!aegis|!aegissingle|!an|!unnuke|!as)' and features ~ '(moderator|admin)' and time >= NOW() - INTERVAL '5 minutes' order by time desc FETCH FIRST 10 ROWS ONLY")
	if err != nil {
		// log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return nil, err
	}
	defer rows1.Close()

	for rows1.Next() {
		p := logLine{}
		err := rows1.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
		if err != nil {
			continue
		}
		logs = append(logs, p)
	}

	rows2, err := pg.Query(context.Background(), "select * from nukes where message ~ 'Dropping the NUKE on' and features ~ '(bot)' and time >= NOW() - INTERVAL '5 minutes' order by time desc FETCH FIRST 10 ROWS ONLY")
	if err != nil {
		// log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		p := logLine{}
		err := rows2.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
		if err != nil {
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
					return data, nil
				case "aegissingle", "an", "unnuke", "as":
					if len(msgSplit) > 1 {
						if len(msgSplit[1]) > 0 {
							matches := regexCheck.FindStringSubmatch(msgSplit[1])
							if len(matches) > 0 {
								unnukeSlice = append(unnukeSlice, fmt.Sprintf("/%s/", matches[2]))
							} else {
								unnukeSlice = append(unnukeSlice, msgSplit[1])
							}
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
					return data, nil
				case "aegissingle", "an", "unnuke", "as":
					if len(msgSplit) > 1 {
						if len(msgSplit[1]) > 0 {
							matches := regexCheck.FindStringSubmatch(msgSplit[1])
							if len(matches) > 0 {
								unnukeSlice = append(unnukeSlice, fmt.Sprintf("/%s/", matches[2]))
							} else {
								unnukeSlice = append(unnukeSlice, msgSplit[1])
							}
						}
					}
					if len(unnukeSlice) > 0 {
						for _, element := range data {
							index := -1
							matches := regexCheck.FindStringSubmatch(element.Word)
							if len(matches) > 0 {
								index = indexOfUnnuke(unnukeSlice, fmt.Sprintf("/%s/", element.Word))
							} else {
								index = indexOfUnnuke(unnukeSlice, element.Word)
							}
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
							regexWord := fmt.Sprintf("/%s/", match[i][2])
							unnukeIndex := indexOfUnnuke(unnukeSlice, regexWord)
							if len(match[i][1]) != 0 {
								if unnukeIndex == -1 {
									if nukedCount != "" {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: match[i][1],
											Word:     regexWord,
											Victims:  nukedCount,
										})
									} else {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: match[i][1],
											Word:     regexWord,
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
											Word:     regexWord,
											Victims:  nukedCount,
										})
									} else {
										data = append(data, nuke{
											Time:     line.Time,
											Type:     nukeType,
											Duration: "10m",
											Word:     regexWord,
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

	return data, nil
}

func phrases(countString string) ([]phrase, error) {
	phrases := []phrase{}

	if countString != "" {
		count, err := strconv.Atoi(countString)
		if err != nil {
			// log.Errorf("[%s] %s %s - String to int conversion error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return nil, err
		}
		rows, err := pg.Query(context.Background(), "select * from phrases order by time desc limit $1", count)
		if err != nil {
			// log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			p := phrase{}
			err := rows.Scan(&p.Time, &p.Username, &p.Phrase, &p.Duration, &p.Type)
			if err != nil {
				// log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			phrases = append(phrases, p)
		}
	} else {
		rows, err := pg.Query(context.Background(), "select * from phrases order by time desc")
		if err != nil {
			// log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			p := phrase{}
			err := rows.Scan(&p.Time, &p.Username, &p.Phrase, &p.Duration, &p.Type)
			if err != nil {
				// log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
				continue
			}
			phrases = append(phrases, p)
		}
	}

	return phrases, nil
}

func mutelinks() ([]fiber.Map, error) {
	logs := []logLine{}

	rows, err := pg.Query(context.Background(), "select * from mutelinks where message ~* '^(!mutelinks|!mutelink|!linkmute|!linksmute)' and features ~ '(moderator|admin)' order by time desc FETCH FIRST 1 ROWS ONLY")
	if err != nil {
		// log.Errorf("[%s] %s %s - Postgres query error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p := logLine{}
		err := rows.Scan(&p.Time, &p.Username, &p.Features, &p.Message)
		if err != nil {
			// log.Errorf("[%s] %s %s - Query scan error: %s", time.Now().Format("2006-01-02 15:04:05.000000 MST"), c.Method(), c.Path()+"?"+string(c.Request().URI().QueryString()), err)
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
						return []fiber.Map{{
							"time":     line.Time,
							"status":   match[i][1],
							"duration": match[i][2],
							"user":     line.Username,
						},
						}, nil
					} else {
						return []fiber.Map{{
							"time":     line.Time,
							"status":   match[i][1],
							"duration": "10m",
							"user":     line.Username,
						},
						}, nil
					}
				}
			}
		}
	}

	return nil, nil
}
