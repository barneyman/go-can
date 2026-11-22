package main

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"go-can/internal/canbus"
	odb_handler "go-can/internal/canbus/PIDhandler"
	Power "go-can/power"
)

type App struct {
	// app fields
	canbus     *canbus.CanbusMonitor
	power      Power.RunMonitor
	pidHandler *odb_handler.PIDHandler
}

func NewApp() *App {
	return &App{}
}

func (app *App) Init() error {

	// use some criteria to decide what power handling we are going to use
	if os.Getenv("POWER") == "rpi" {
		app.power = Power.NewRPICarPiHatPower(time.Minute, time.Second*5)
	} else {
		app.power = Power.NewNormalPower()
	}

	err := app.power.Init()
	if err != nil {
		return err
	}

	// then work out what canbus src & dst we are going to use
	app.canbus = canbus.NewCanbusMonitor()

	canbussrcType := canbus.UnknownSrc
	canbusdstType := canbus.UnknownDst

	namedDst := os.Getenv("CANDEST")

	switch namedDst {
	case "archive":
		canbusdstType = canbus.CbmDstarchive
	case "null":
		canbusdstType = canbus.CbmDstNull
	default:
		return errors.New("no CANDEST requested")
	}

	namedSrc := os.Getenv("CANSRC")

	listenOnly := false
	if os.Getenv("LISTEN_ONLY") != "" {
		listenOnly = true
	}

	switch namedSrc {
	case "native":
		canbussrcType = canbus.CbmSrcNamedSource
	case "archive":
		canbussrcType = canbus.CbmSrcarchive
	case "test":
		canbussrcType = canbus.CbmSrcTest
	case "vcan":
		canbussrcType = canbus.CbmSrcVirtual
	default:
		return errors.New("no CANSRC requested")
	}

	err = app.canbus.Init(canbussrcType, canbusdstType, nil, nil)
	if err != nil {
		return err
	}

	// then create a pidhandler, and give it the canbus source and destination
	app.pidHandler = odb_handler.NewPIDHandler()
	app.pidHandler.Init(app.canbus, listenOnly)

	// should we spin on anything?

	return nil
}

func (app *App) Run() error {
	// run the app
	defer app.power.Close()
	defer app.canbus.Close()
	defer app.pidHandler.Close()

	// create our context
	ctx, cancel := context.WithCancel(context.Background())
	// and a wait group
	wg := &sync.WaitGroup{}

	wg.Go(func() {
		app.power.Run(ctx, wg)
	})

	wg.Go(func() {
		app.canbus.Run(ctx, wg)
	})

	wg.Go(func() {
		app.pidHandler.Run(ctx, wg)
	})

	for running := true; running; {

		select {
		case <-app.power.GetExitPowerChannel():
			// handle power exit request
			running = false
		default:
			// do nothing, just continue looping

		}

	}
	log.Print("App.Run exiting - calling Cancel")
	// fire the cancel
	cancel()
	log.Print("Waiting ...")
	wg.Wait()
	log.Print("Out")
	return nil
}
