package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/syahrulfahmi/kolaboria-api/config"
	"github.com/syahrulfahmi/kolaboria-api/internal/http"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/database"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/logger"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/mail"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/auth"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/health"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/master"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/profile"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/workspace"
	"gorm.io/gorm"
)

type Application struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *gorm.DB
	Server *Server
}

func New() *Application {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 2. Initialize logger
	appLogger := logger.New(cfg.App.Env)

	// 3. Connect to database
	db, err := database.NewPostgresConnection(cfg.Database, cfg.App.Debug)
	if err != nil {
		appLogger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	// 4. Initialize Handlers & Services
	healthHandler := health.NewHealthHandler()
	masterRepository := master.NewMasterRepository(db)
	masterService := master.NewMasterService(masterRepository)
	masterHandler := master.NewMasterHandler(masterService)
	profileRepository := profile.NewProfileRepository(db)
	profileService := profile.NewProfileService(profileRepository)
	profileHandler := profile.NewProfileHandler(profileService)
	projectRepository := project.NewProjectRepository(db)
	projectService := project.NewProjectService(projectRepository)
	applicationRepository := project.NewApplicationRepository(db)
	applicationService := project.NewApplicationService(applicationRepository, nil)
	applicationHandler := project.NewApplicationHandler(applicationService)
	projectHandler := project.NewProjectHandler(projectService, applicationHandler)
	workspaceRepository := workspace.NewRepository(db)
	workspaceService := workspace.NewService(workspaceRepository)
	workspaceTeamRepository := workspace.NewTeamRepository(db)
	workspaceTeamService := workspace.NewTeamService(workspaceTeamRepository)
	workspaceDiscussionRepository := workspace.NewDiscussionRepository(db)
	workspaceDiscussionService := workspace.NewDiscussionService(workspaceDiscussionRepository)
	workspaceTaskRepository := workspace.NewTaskRepository(db)
	workspaceTaskService := workspace.NewTaskService(workspaceTaskRepository)
	workspaceActivityService := workspace.NewActivityService(workspace.NewActivityRepository(db))
	workspaceHandler := workspace.NewHandlerWithActivities(workspaceService, workspaceTeamService, workspaceTaskService, workspaceDiscussionService, workspaceActivityService)
	workspaceHandler.WithCompletion(workspace.NewCompletionService(workspace.NewCompletionRepository(db)))
	requestTimeout, snapshotTimeout, statementTimeout, lockTimeout := cfg.Workspace.OverviewDurations()
	overviewRepository := workspace.NewOverviewRepository(db, workspace.OverviewReadOptions{StatementTimeout: statementTimeout, LockTimeout: lockTimeout})
	overviewRepository = workspace.NewOverviewCoordinator(overviewRepository, cfg.Workspace.OverviewMaxSnapshots, snapshotTimeout)
	workspaceHandler.WithOverview(workspace.NewOverviewService(overviewRepository), workspace.NewOverviewAdmission(cfg.Workspace.OverviewEnabled, cfg.Workspace.OverviewMaxRequests, requestTimeout))

	jwtService := jwt.NewJWTService(cfg.JWT)
	mailer := mail.NewSMTPMailer(cfg.Mail, cfg.App.FrontendURL)
	authRepository := auth.NewAuthRepository(db)
	authService := auth.NewAuthService(authRepository, jwtService, mailer, cfg)
	authHandler := auth.NewAuthHandler(authService)

	// 5. Initialize Router
	router := http.NewRouter(cfg, healthHandler, jwtService, masterHandler, authHandler, profileHandler, projectHandler, workspaceHandler)

	// 6. Initialize Server
	server := NewServer(cfg.App, router)

	return &Application{
		Config: cfg,
		Logger: appLogger,
		DB:     db,
		Server: server,
	}
}

func (app *Application) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	app.Server.Start(ctx)
}
