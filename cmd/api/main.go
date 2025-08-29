package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"sol-api/internal/cache"
	"sol-api/internal/database"
	"sol-api/internal/ratelimit"
	"sol-api/internal/sol"
)

type GetBalanceRequest struct {
	Wallets []string `json:"wallets"`
}

type GetBalanceResponse struct {
	Balances []sol.WalletBalance `json:"balances"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

var (
	balanceService *sol.BalanceService
	db             *database.DB
	apiKeyCache    *cache.Cache
	rateLimiter    *ratelimit.RateLimiter
)

func main() {
	var err error
	db, err = database.New("database.sqlite")
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		panic(err)
	}
	defer db.Close()

	endpoint := "https://pomaded-lithotomies-xfbhnqagbt-dedicated.helius-rpc.com/?api-key=37ba4475-8fa3-4491-875f-758894981943"
	balanceService = sol.NewBalanceService(endpoint)
	apiKeyCache = cache.New()
	rateLimiter = ratelimit.New(10, time.Minute)

	slog.Info("initialized services", "endpoint", endpoint)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	api := e.Group("/api")
	api.Use(rateLimitMiddleware)
	api.Use(authMiddleware)

	api.POST("/get-balance", getBalance)

	slog.Info("starting server on :8081")
	if err := e.Start(":8081"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}

func rateLimitMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := c.RealIP()
		if ip == "" {
			ip = c.Request().RemoteAddr
		}

		if !rateLimiter.Allow(ip) {
			return c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error: "rate limit exceeded (10 requests per minute)",
			})
		}

		return next(c)
	}
}

func authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		apiKey := c.Request().Header.Get("api-key")
		if apiKey == "" {
			return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "API key required"})
		}

		cacheKey := "apikey:" + apiKey
		if cached, found := apiKeyCache.Get(cacheKey); found {
			if valid, ok := cached.(bool); ok && valid {
				return next(c)
			}
		}

		valid, err := db.ValidateAPIKey(apiKey)
		if err != nil {
			slog.Error("failed to validate API key", "error", err)
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}

		if !valid {
			return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid API key"})
		}

		apiKeyCache.Set(cacheKey, true, 600*time.Second)
		return next(c)
	}
}

func getBalance(c echo.Context) error {
	var req GetBalanceRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	if len(req.Wallets) == 0 {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "no wallets provided"})
	}

	for _, wallet := range req.Wallets {
		wallet = strings.TrimSpace(wallet)
		if !isValidWallet(wallet) {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid wallet address format"})
		}
	}

	slog.Info("processing balance request", "wallets", len(req.Wallets))

	ctx := c.Request().Context()
	balances := balanceService.GetBalances(ctx, req.Wallets)

	return c.JSON(http.StatusOK, GetBalanceResponse{Balances: balances})
}

func isValidWallet(wallet string) bool {
	return len(wallet) >= 32 && len(wallet) <= 44
}
