package main

import "github.com/syahrulfahmi/kolaboria-api/internal/bootstrap"

// @title           Kolaboria API
// @version         1.0
// @description     Backend API for Kolaboria Platform
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Login via POST /api/v1/auth/login, copy data.access_token, then enter Bearer <access_token> here before calling protected endpoints.
func main() {
	app := bootstrap.New()
	app.Run()
}
