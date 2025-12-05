package l1

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/l1/contract"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

var finalizedBlockNumber = new(big.Int).SetInt64(rpc.FinalizedBlockNumber.Int64())

type EthSubscriber struct {
	ethClient *ethclient.Client
	client    *rpc.Client
	filterer  *contract.StarknetFilterer
	listener  EventListener
}

var _ Subscriber = (*EthSubscriber)(nil)

// uniswapHeaderTransport wraps an http.RoundTripper to add Uniswap headers for Infura compatibility
type uniswapHeaderTransport struct {
	transport http.RoundTripper
}

func (t *uniswapHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add Uniswap CORS headers for Infura compatibility
	req.Header.Set("Origin", "https://app.uniswap.org")
	req.Header.Set("Referer", "https://app.uniswap.org/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
	return t.transport.RoundTrip(req)
}

func NewEthSubscriber(ethClientAddress string, coreContractAddress common.Address) (*EthSubscriber, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Create custom HTTP client with Uniswap headers for Infura compatibility
	httpClient := &http.Client{
		Transport: &uniswapHeaderTransport{
			transport: http.DefaultTransport,
		},
	}

	client, err := rpc.DialOptions(ctx, ethClientAddress, rpc.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	ethClient := ethclient.NewClient(client)
	filterer, err := contract.NewStarknetFilterer(coreContractAddress, ethClient)
	if err != nil {
		return nil, err
	}
	return &EthSubscriber{
		ethClient: ethClient,
		client:    client,
		filterer:  filterer,
		listener:  SelectiveListener{},
	}, nil
}

func (s *EthSubscriber) WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error) {
	return s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, sink)
}

func (s *EthSubscriber) ChainID(ctx context.Context) (*big.Int, error) {
	reqTimer := time.Now()
	chainID, err := s.ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain ID: %w", err)
	}
	s.listener.OnL1Call("eth_chainId", time.Since(reqTimer))

	return chainID, nil
}

func (s *EthSubscriber) FinalisedHeight(ctx context.Context) (uint64, error) {
	reqTimer := time.Now()
	head, err := s.ethClient.HeaderByNumber(ctx, finalizedBlockNumber)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			s.listener.OnL1Call("eth_getBlockByNumber", time.Since(reqTimer))
			return 0, errors.New("finalised block not found")
		}
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}
	s.listener.OnL1Call("eth_getBlockByNumber", time.Since(reqTimer))

	return head.Number.Uint64(), nil
}

func (s *EthSubscriber) LatestHeight(ctx context.Context) (uint64, error) {
	reqTimer := time.Now()
	height, err := s.ethClient.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("get latest Ethereum block number: %w", err)
	}
	s.listener.OnL1Call("eth_blockNumber", time.Since(reqTimer))

	return height, nil
}

func (s *EthSubscriber) Close() {
	s.ethClient.Close()
}

func (s *EthSubscriber) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	reqTimer := time.Now()
	receipt, err := s.ethClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("get eth Transaction Receipt: %w", err)
	}
	s.listener.OnL1Call("eth_getTransactionReceipt", time.Since(reqTimer))

	return receipt, nil
}
