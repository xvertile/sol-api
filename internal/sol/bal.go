package sol

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type BalanceService struct {
	client *rpc.Client
}

type WalletBalance struct {
	Wallet  string  `json:"wallet"`
	Balance float64 `json:"balance"`
	Error   string  `json:"error,omitempty"`
}

func NewBalanceService(endpoint string) *BalanceService {
	return &BalanceService{
		client: rpc.New(endpoint),
	}
}

func (s *BalanceService) GetBalances(ctx context.Context, wallets []string) []WalletBalance {
	results := make([]WalletBalance, len(wallets))
	var wg sync.WaitGroup
	
	for i, wallet := range wallets {
		wg.Add(1)
		go func(index int, walletAddr string) {
			defer wg.Done()
			
			result := WalletBalance{
				Wallet: walletAddr,
			}
			
			pubKey, err := solana.PublicKeyFromBase58(walletAddr)
			if err != nil {
				result.Error = fmt.Sprintf("invalid wallet address: %v", err)
				results[index] = result
				return
			}
			
			out, err := s.client.GetBalance(
				ctx,
				pubKey,
				rpc.CommitmentFinalized,
			)
			if err != nil {
				result.Error = fmt.Sprintf("failed to get balance: %v", err)
				results[index] = result
				return
			}
			
			lamportsOnAccount := new(big.Float).SetUint64(uint64(out.Value))
			solBalance := new(big.Float).Quo(lamportsOnAccount, new(big.Float).SetUint64(solana.LAMPORTS_PER_SOL))
			
			balance, _ := solBalance.Float64()
			result.Balance = balance
			results[index] = result
		}(i, wallet)
	}
	
	wg.Wait()
	return results
}