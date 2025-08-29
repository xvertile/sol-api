package main

import (
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"log/slog"
	"net/http"
	"sol-api/internal/sol"
)

type GetBalanceRequest struct {
	Wallets []string `json:"wallets"`
}

type GetBalanceResponse struct {
	Balances []sol.WalletBalance `json:"balances"`
}

var balanceService *sol.BalanceService

func main() {
	endpoint := "https://pomaded-lithotomies-xfbhnqagbt-dedicated.helius-rpc.com/?api-key=37ba4475-8fa3-4491-875f-758894981943"
	balanceService = sol.NewBalanceService(endpoint)
	slog.Info("initialized balance service", "endpoint", endpoint)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.POST("/api/get-balance", getBalance)

	slog.Info("starting server on :8081")
	if err := e.Start(":8081"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}

func getBalance(c echo.Context) error {
	var req GetBalanceRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if len(req.Wallets) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no wallets provided"})
	}

	slog.Info("processing balance request", "wallets", len(req.Wallets))

	ctx := c.Request().Context()
	balances := balanceService.GetBalances(ctx, req.Wallets)

	return c.JSON(http.StatusOK, GetBalanceResponse{
		Balances: balances,
	})
}
