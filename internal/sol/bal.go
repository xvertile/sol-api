package sol

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"sol-api/internal/cache"
)

type BalanceService struct {
	client          *rpc.Client
	cache           *cache.Cache
	walletMutexes   map[string]*sync.Mutex
	walletMutexLock sync.RWMutex
}

type WalletBalance struct {
	Wallet  string  `json:"wallet"`
	Balance float64 `json:"balance"`
	Error   string  `json:"error,omitempty"`
}

func NewBalanceService(endpoint string) *BalanceService {
	return &BalanceService{
		client:        rpc.New(endpoint),
		cache:         cache.New(),
		walletMutexes: make(map[string]*sync.Mutex),
	}
}

func (s *BalanceService) getWalletMutex(wallet string) *sync.Mutex {
	s.walletMutexLock.Lock()
	defer s.walletMutexLock.Unlock()

	if mutex, exists := s.walletMutexes[wallet]; exists {
		return mutex
	}

	mutex := &sync.Mutex{}
	s.walletMutexes[wallet] = mutex
	return mutex
}

func (s *BalanceService) GetBalances(ctx context.Context, wallets []string) []WalletBalance {
	results := make([]WalletBalance, len(wallets))
	var wg sync.WaitGroup

	for i, wallet := range wallets {
		wg.Add(1)
		go func(index int, walletAddr string) {
			defer wg.Done()
			results[index] = s.getBalance(ctx, walletAddr)
		}(i, wallet)
	}

	wg.Wait()
	return results
}

func (s *BalanceService) getBalance(ctx context.Context, walletAddr string) WalletBalance {

	cacheKey := fmt.Sprintf("balance:%s", walletAddr)
	if cached, found := s.cache.Get(cacheKey); found {
		if balance, ok := cached.(WalletBalance); ok {
			return balance
		}
	}

	mutex := s.getWalletMutex(walletAddr)
	mutex.Lock()
	defer mutex.Unlock()

	if cached, found := s.cache.Get(cacheKey); found {
		if balance, ok := cached.(WalletBalance); ok {
			return balance
		}
	}

	result := WalletBalance{
		Wallet: walletAddr,
	}

	pubKey, err := solana.PublicKeyFromBase58(walletAddr)
	if err != nil {
		result.Error = fmt.Sprintf("invalid wallet address: %v", err)
		return result
	}

	out, err := s.client.GetBalance(
		ctx,
		pubKey,
		rpc.CommitmentFinalized,
	)
	if err != nil {

		result.Balance = 0
	} else {
		lamportsOnAccount := new(big.Float).SetUint64(uint64(out.Value))
		solBalance := new(big.Float).Quo(lamportsOnAccount, new(big.Float).SetUint64(solana.LAMPORTS_PER_SOL))
		balance, _ := solBalance.Float64()
		result.Balance = balance
	}

	s.cache.Set(cacheKey, result, 10*time.Second)

	return result
}
