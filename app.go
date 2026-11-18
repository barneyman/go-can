package main

type App struct {
	// app fields
}

func NewApp() *App {
	return &App{}
}

func (app *App) Init() error {

	// use some criteria to decide what power handling we are going to use
	// currently rpicarpihat

	return nil
}

func (app *App) Run() error {
	// run the app
	return nil
}
