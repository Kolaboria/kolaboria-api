package main

import "github.com/syahrulfahmi/kolaboria-api/internal/bootstrap"

// @title           Kolaboria API
// @version         1.0
// @description     Backend API for Kolaboria Platform
// @BasePath        /
func main() {
	app := bootstrap.New()
	app.Run()
}
