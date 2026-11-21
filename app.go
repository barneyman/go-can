package main

import (
	"context"
	"log"
	"sync"

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
	// currently rpicarpihat
	app.power = Power.NewNormalPower()
	app.power.Init()

	// then work out what canbus src & dst we are going to use
	app.canbus = canbus.NewCanbusMonitor()

	app.canbus.Init(canbus.CbmSrcVirtual, canbus.CbmDstarchive, nil, nil)

	// then create a pidhandler, and give it the canbus source and destination
	app.pidHandler = odb_handler.NewPIDHandler()
	app.pidHandler.Init(app.canbus)

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

	go app.power.Run(ctx, wg)

	go app.canbus.Run(ctx, wg)

	go app.pidHandler.Run(ctx, wg)

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
