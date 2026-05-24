package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/pressly/goose/v3"

	"restaurant/backend/internal/config"
	"restaurant/backend/internal/database"
	"restaurant/backend/internal/realtime"
	"restaurant/backend/internal/repository"
	"restaurant/backend/internal/service"
	httptransport "restaurant/backend/internal/transport/http"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(db, "db/migrations"); err != nil {
		log.Fatal(err)
	}

	hub := realtime.NewHub(logger)
	repo := repository.New(db)
	app := service.New(repo, hub, cfg.JWTSecret)
	handler := httptransport.New(app, hub, cfg)
     go func(){
		_,err:=http.Get("https://restaurantbackend-w0jj.onrender.com")
		if err!=nil{
			log.Fatal(err)
		}
		time.Sleep(5* time.Minute)
	 }()
	logger.Info("api listening", "addr", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler.Routes(httptransport.Logging(logger))); err != nil {
		log.Fatal(err)
	}
}
