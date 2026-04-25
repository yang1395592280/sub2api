package main
import (
  "context"
  "fmt"
  entsql "entgo.io/ent/dialect/sql"
  _ "github.com/lib/pq"
  dbent "github.com/Wei-Shaw/sub2api/ent"
  "github.com/Wei-Shaw/sub2api/internal/config"
  "github.com/Wei-Shaw/sub2api/internal/repository"
  "github.com/Wei-Shaw/sub2api/internal/service"
)
func main() {
  cfg, err := config.Load()
  if err != nil { panic(err) }
  dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
  drv, err := entsql.Open("postgres", dsn)
  if err != nil { panic(err) }
  defer drv.Close()
  client := dbent.NewClient(dbent.Driver(drv))
  defer client.Close()
  repo := repository.NewSettingRepository(client)
  svc := service.NewSettingService(repo, cfg)
  ps, err := svc.GetPublicSettings(context.Background())
  if err != nil { panic(err) }
  fmt.Printf("GameCenterEnabled=%v\n", ps.GameCenterEnabled)
}
