package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"subdeck/internal/auth"
	"subdeck/internal/config"
	"subdeck/internal/database"
	"subdeck/internal/handlers"
	"subdeck/internal/middleware"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
)

func setupRouter(db *database.DB, fetcher *services.FetcherService, distSub fs.FS, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Printf("Warning: invalid trusted_proxies: %v\n", err)
	}

	corsOrigin := cfg.Server.CorsOrigin
	if corsOrigin == "" {
		corsOrigin = "*"
	}

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", corsOrigin)
		if corsOrigin != "*" {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization, Accept, Origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Handlers
	authH := handlers.NewAuthHandler(db)
	dashH := handlers.NewDashboardHandler(db)
	subH := handlers.NewSubscriptionHandler(db, fetcher)
	profH := handlers.NewProfileHandler(db, fetcher)
	exportH := handlers.NewExportHandler(db, fetcher)
	sysH := handlers.NewSystemHandler(db)

	// Public Routes
	r.POST("/api/auth/login", authH.Login)
	r.GET("/api/sub/:token", middleware.SubscriptionRateLimitMiddleware(), exportH.ExportSubscription)
	r.GET("/s/:token", middleware.SubscriptionRateLimitMiddleware(), exportH.ExportSubscription)

	// Protected Admin Routes
	api := r.Group("/api")
	api.Use(middleware.AuthRequired())
	{
		api.GET("/auth/me", authH.Me)
		api.POST("/auth/password", authH.UpdatePassword)
		api.POST("/auth/logout-all", authH.LogoutAll)

		api.GET("/dashboard/stats", dashH.GetStats)

		// System Backup, Restore & Settings
		api.GET("/system/backup", sysH.DownloadBackup)
		api.POST("/system/restore", sysH.RestoreBackup)
		api.GET("/system/settings", sysH.GetSettings)
		api.PUT("/system/settings", sysH.UpdateSettings)
		api.POST("/system/webhook-test", sysH.TestWebhook)

		// Subscriptions (both Remote and Local)
		api.GET("/subscriptions", subH.List)
		api.POST("/subscriptions", subH.Create)
		api.PUT("/subscriptions/:id", subH.Update)
		api.DELETE("/subscriptions/:id", subH.Delete)
		api.POST("/subscriptions/:id/refresh", subH.Refresh)
		api.POST("/subscriptions/:id/probe", subH.Probe)
		api.GET("/subscriptions/:id/nodes", subH.GetNodes)

		// Profiles (Combinations)
		api.GET("/profiles", profH.List)
		api.POST("/profiles", profH.Create)
		api.POST("/profiles/preview", profH.PreviewDraft)
		api.PUT("/profiles/:id", profH.Update)
		api.DELETE("/profiles/:id", profH.Delete)
		api.POST("/profiles/:id/reset-token", profH.ResetToken)
		api.POST("/profiles/:id/refresh-templates", profH.RefreshTemplates)
		api.POST("/profiles/:id/probe", profH.Probe)
		api.GET("/profiles/:id/preview", profH.Preview)
	}

	// Embedded Static Frontend Serving
	if distSub != nil {
		fileServer := http.FileServer(http.FS(distSub))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
				return
			}
			f, err := distSub.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			indexBytes, err := fs.ReadFile(distSub, "index.html")
			if err == nil {
				c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
				return
			}
			c.String(http.StatusNotFound, "Frontend UI not found")
		})
	}

	return r
}

func main() {
	configPath := flag.String("config", "", "YAML config file path (or SUBDECK_CONFIG env, defaults to config.yaml if present)")
	initConfig := flag.Bool("init-config", false, "Write default config file to -config path and exit")
	showVersion := flag.Bool("version", false, "Print SubDeck version and exit")
	host := flag.String("host", "0.0.0.0", "Web server listen host")
	port := flag.Int("port", 8080, "Web server port")
	dbPath := flag.String("db", "data/subs.db", "SQLite database file path")
	secret := flag.String("secret", "", "JWT secret key (or SUBDECK_JWT_SECRET env)")
	adminUser := flag.String("admin-user", "", "Admin username (overrides config)")
	adminPass := flag.String("admin-pass", "", "Admin password (overrides config)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("SubDeck %s\n", config.Version)
		return
	}

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	cfgFile := *configPath
	explicitConfig := explicitFlags["config"]
	if cfgFile == "" {
		if envCfg := os.Getenv("SUBDECK_CONFIG"); envCfg != "" {
			cfgFile = envCfg
			explicitConfig = true
		} else {
			cfgFile = config.DefaultConfigFile
		}
	}

	if *initConfig {
		if err := config.WriteDefault(cfgFile); err != nil {
			log.Fatalf("Failed to write config file %s: %v\n", cfgFile, err)
		}
		fmt.Printf("Default config written to %s\n", cfgFile)
		return
	}

	cfg, err := config.Load(cfgFile, explicitConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v\n", err)
	}

	// CLI flags take highest precedence
	if explicitFlags["host"] {
		cfg.Server.Host = *host
	}
	if explicitFlags["port"] {
		cfg.Server.Port = *port
	}
	if explicitFlags["db"] {
		cfg.Database.Path = *dbPath
	}
	if explicitFlags["secret"] {
		cfg.Auth.JWTSecret = *secret
	}
	if explicitFlags["admin-user"] && strings.TrimSpace(*adminUser) != "" {
		cfg.Auth.AdminUsername = strings.TrimSpace(*adminUser)
	}
	if explicitFlags["admin-pass"] {
		cfg.Auth.AdminPassword = *adminPass
	}

	// 1. Initialize SQLite Database & Sync Admin Account
	db, usedDefaultPass, err := database.InitDBWithAdmin(cfg.Database.Path, cfg.Auth.AdminUsername, cfg.Auth.AdminPassword)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v\n", err)
	}
	defer db.Close()
	log.Printf("Connected to SQLite at %s\n", cfg.Database.Path)

	// 2. Configure JWT Secret (flag -> env -> config -> persisted random secret in SQLite)
	jwtSecret := cfg.Auth.JWTSecret
	if jwtSecret == "" {
		jwtSecret, err = db.GetOrCreateJWTSecret()
		if err != nil {
			log.Fatalf("Failed to initialize JWT secret: %v\n", err)
		}
	}
	auth.SetJWTSecret(jwtSecret)
	auth.SetTokenVersion(db.GetTokenVersion())

	// 3. Initialize Fetcher & Background Scheduler
	fetcher := services.NewFetcherService(db)
	scheduler := services.NewScheduler(db, fetcher)
	scheduler.Start()
	defer scheduler.Stop()

	// 4. Setup HTTP Engine & Graceful Shutdown
	distSub, _ := getFrontendDistFS()
	r := setupRouter(db, fetcher, distSub, cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	adminHint := cfg.Auth.AdminUsername
	if usedDefaultPass {
		adminHint = fmt.Sprintf("%s / admin123 (default, please change!)", cfg.Auth.AdminUsername)
	}

	fmt.Printf(`
=====================================================
   SubDeck %s - VPN Subscription Control Plane
=====================================================
 Server listening on: http://%s
 SQLite Database:     %s
 Admin Account:       %s
=====================================================
`, config.Version, addr, cfg.Database.Path, adminHint)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down SubDeck server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v\n", err)
	}
}
