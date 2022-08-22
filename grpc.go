package main

import (
	"context"
	"net"
	"time"
	_ "time/tzdata"

	"github.com/gofiber/fiber/v2"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/vyneer/vyneer-api/logger"
	"github.com/vyneer/vyneer-api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	proto.UnimplementedStatusServer
}

var phraseStamp int64 = 0
var phraseRemovalStamp int64 = 0
var nukeStamp int64 = 0
var mutelinksStamp int64 = 0

func (s *server) ReceiveRemovePhrase(ctx context.Context, in *proto.RemovePhrase) (*proto.Empty, error) {
	newStamp := in.Time.AsTime().UnixMilli()
	log.Infof("Received a gRPC phrase removal event, updating the phraseRemovalStamp variable: %+v -> %+v", phraseRemovalStamp, newStamp)
	phraseRemovalStamp = newStamp
	return &proto.Empty{}, nil
}

func (s *server) ReceivePhrase(ctx context.Context, in *proto.Phrase) (*proto.Empty, error) {
	newStamp := in.Time.AsTime().UnixMilli()
	log.Infof("Received a gRPC phrase event, updating the phraseStamp variable: %+v -> %+v", phraseStamp, newStamp)
	phraseStamp = newStamp
	return &proto.Empty{}, nil
}

func (s *server) ReceiveNuke(ctx context.Context, in *proto.Nuke) (*proto.Empty, error) {
	newStamp := in.Time.AsTime().UnixMilli()
	log.Infof("Received a gRPC nuke event, updating the nukeStamp variable: %+v -> %+v", nukeStamp, newStamp)
	nukeStamp = newStamp
	go func() {
		time.Sleep(time.Minute * 5)
		nukeStamp = time.Now().UnixMilli()
	}()
	return &proto.Empty{}, nil
}

func (s *server) ReceiveAegis(ctx context.Context, in *proto.Aegis) (*proto.Empty, error) {
	newStamp := in.Time.AsTime().UnixMilli()
	log.Infof("Received a gRPC aegis event, updating the nukeStamp variable: %+v -> %+v", nukeStamp, newStamp)
	nukeStamp = newStamp
	return &proto.Empty{}, nil
}

func (s *server) ReceiveMutelinks(ctx context.Context, in *proto.Mutelinks) (*proto.Empty, error) {
	newStamp := in.Time.AsTime().UnixMilli()
	log.Infof("Received a gRPC nuke event, updating the mutelinksStamp variable: %+v -> %+v", mutelinksStamp, newStamp)
	mutelinksStamp = newStamp
	return &proto.Empty{}, nil
}

func gRPCServer() {
	listener, err := net.Listen("tcp", ":6413")
	if err != nil {
		log.Fatalf("Couldn't create gRPC server", time.Now().Format("2006-01-02 15:04:05.000000 MST"))
	}

	s := grpc.NewServer()
	reflection.Register(s)
	proto.RegisterStatusServer(s, &server{})
	log.Infof("Starting a gRPC server on port 6413")
	if err := s.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", time.Now().Format("2006-01-02 15:04:05.000000 MST"), err)
	}
}

func doubleCheckStamps() error {
	rowPhrase := pg.QueryRow(context.Background(), "select time from phrases order by time desc limit 1;")
	phraseStampInner := logLine{}
	err := rowPhrase.Scan(&phraseStampInner.Time)
	if err != nil {
		phraseStampInner.Time = time.Unix(0, 0)
	}

	rowNuke := pg.QueryRow(context.Background(), "select time from nukes where username != 'Bot' order by time desc limit 1;")
	nukeStampInner := logLine{}
	err = rowNuke.Scan(&nukeStampInner.Time)
	if err != nil {
		nukeStampInner.Time = time.Unix(0, 0)
	}

	rowMutelinks := pg.QueryRow(context.Background(), "select time from mutelinks order by time desc limit 1;")
	mutelinksStampInner := logLine{}
	err = rowMutelinks.Scan(&mutelinksStampInner.Time)
	if err != nil {
		mutelinksStampInner.Time = time.Unix(0, 0)
	}

	phraseStampInnerMilli := phraseStampInner.Time.UnixMilli()
	nukeStampInnerMilli := nukeStampInner.Time.UnixMilli()
	mutelinksStampInnerMilli := mutelinksStampInner.Time.UnixMilli()

	if phraseStampInnerMilli != phraseStamp {
		log.Infof("Updating the phraseStamp variable with the proper timestamp: %+v -> %+v", phraseStamp, phraseStampInner.Time.UnixMilli())
		phraseStamp = phraseStampInnerMilli
	}

	if nukeStampInnerMilli > nukeStamp {
		log.Infof("Updating the nukeStamp variable with the proper timestamp: %+v -> %+v", nukeStamp, nukeStampInner.Time.UnixMilli())
		nukeStamp = nukeStampInnerMilli
	}

	if mutelinksStampInnerMilli != mutelinksStamp {
		log.Infof("Updating the mutelinksStamp variable with the proper timestamp: %+v -> %+v", mutelinksStamp, mutelinksStampInner.Time.UnixMilli())
		mutelinksStamp = mutelinksStampInnerMilli
	}

	return nil
}

func checkStamps(c *fiber.Ctx) error {
	if phraseStamp >= phraseRemovalStamp {
		return c.JSON(fiber.Map{
			"phrases":   phraseStamp,
			"nukes":     nukeStamp,
			"mutelinks": mutelinksStamp,
		})
	} else {
		return c.JSON(fiber.Map{
			"phrases":   phraseRemovalStamp,
			"nukes":     nukeStamp,
			"mutelinks": mutelinksStamp,
		})
	}
}
