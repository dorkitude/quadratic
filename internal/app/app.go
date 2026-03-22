package app

import (
	"context"

	"quadratic/internal/browse"
	"quadratic/internal/config"
	"quadratic/internal/foursquare"
	"quadratic/internal/store"
	"quadratic/internal/tui"
)

type App struct {
	cfg    *config.Config
	client *foursquare.Client
	store  *store.Store
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	st, err := store.New(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	client := foursquare.NewClient(cfg)
	return &App{cfg: cfg, client: client, store: st}, nil
}

func (a *App) Login(ctx context.Context) (string, error) {
	token, err := a.client.Login(ctx)
	if err != nil {
		return "", err
	}

	a.cfg.AccessToken = token
	if err := config.Save(a.cfg); err != nil {
		return "", err
	}
	return token, nil
}

func (a *App) Sync(ctx context.Context) (*store.SyncResult, error) {
	return a.store.SyncCheckins(ctx, a.client)
}

func (a *App) NewModel(ctx context.Context) (tui.Model, error) {
	summary, err := a.store.Summary()
	if err != nil {
		return tui.Model{}, err
	}
	summary.TokenPresent = a.cfg.AccessToken != ""
	model := tui.NewModel(ctx, a.cfg, a.client, a.store, summary).WithActions(a.Login, a.Sync)
	return model, nil
}

func (a *App) Browse(ctx context.Context) error {
	if err := a.store.PrepareArchive(ctx); err != nil {
		return err
	}
	server := browse.New(a.store)
	return server.Run(ctx)
}
