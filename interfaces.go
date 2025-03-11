// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package ethereum provides interfaces for interacting with the Ethereum blockchain.
// It defines the core abstractions for blockchain access, transaction handling,
// state queries, and event subscriptions.
package ethereum

import (
    "context"
    "errors"
    "math/big"

    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
)

// ErrNotFound is returned by API methods when the requested item does not exist.
var ErrNotFound = errors.New("not found")

// Subscription represents an event subscription delivering events through a channel.
type Subscription interface {
    // Unsubscribe cancels event delivery and closes the error channel.
    Unsubscribe()
    // Err returns the subscription error channel, receiving a single error value
    // when subscription issues occur (e.g., network disconnection).
    // The channel is closed upon unsubscribing.
    Err() <-chan error
}

// ChainReader provides read-only access to blockchain data.
// It retrieves raw data from either the canonical chain (by block number)
// or previously processed forks. Use nil block number for the latest canonical block.
// Header queries are preferred over full blocks when possible.
// All methods return ErrNotFound if the requested item is unavailable.
type ChainReader interface {
    // BlockByHash retrieves a block by its hash.
    BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
    // BlockByNumber retrieves a block by its number (nil for latest).
    BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
    // HeaderByHash retrieves a block header by its hash.
    HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error)
    // HeaderByNumber retrieves a block header by its number (nil for latest).
    HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
    // TransactionCount returns the number of transactions in a block.
    TransactionCount(ctx context.Context, blockHash common.Hash) (uint, error)
    // TransactionInBlock retrieves a specific transaction from a block.
    TransactionInBlock(ctx context.Context, blockHash common.Hash, index uint) (*types.Transaction, error)
    // SubscribeNewHead subscribes to new canonical chain head notifications.
    SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (Subscription, error)
}

// TransactionReader provides access to historical transactions and receipts.
// Implementations may limit availability of past data.
// Prefer LogFilterer for reliability during chain reorganizations.
// Methods return ErrNotFound if items don't exist.
type TransactionReader interface {
    // TransactionByHash retrieves a transaction, checking both pending pool
    // and blockchain. Returns whether the transaction is pending.
    TransactionByHash(ctx context.Context, txHash common.Hash) (tx *types.Transaction, isPending bool, err error)
    // TransactionReceipt retrieves a mined transaction's receipt.
    // Receipt may exist even if not in canonical chain.
    TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

// ChainStateReader provides access to the canonical blockchain's state trie.
// Older block states may be unavailable; consider CallContract for contract storage access.
type ChainStateReader interface {
    BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
    StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
    CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error)
    NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
}

// SyncProgress tracks blockchain synchronization progress with the Ethereum network.
type SyncProgress struct {
    StartingBlock uint64 // Block number where sync started
    CurrentBlock  uint64 // Current synced block number
    HighestBlock  uint64 // Highest known block number

    // Snap sync metrics
    SyncedAccounts      uint64 // Number of downloaded accounts
    SyncedAccountBytes  uint64 // Bytes of account trie data on disk
    SyncedBytecodes     uint64 // Number of downloaded bytecodes
    SyncedBytecodeBytes uint64 // Bytes of bytecode data downloaded
    SyncedStorage       uint64 // Number of downloaded storage slots
    SyncedStorageBytes  uint64 // Bytes of storage trie data on disk

    // Healing metrics
    HealedTrienodes     uint64 // Number of healed state trie nodes
    HealedTrienodeBytes uint64 // Bytes of healed trie data
    HealedBytecodes     uint64 // Number of healed bytecodes
    HealedBytecodeBytes uint64 // Bytes of healed bytecode data

    HealingTrienodes uint64 // Pending state trie nodes
    HealingBytecode  uint64 // Pending bytecodes

    // Transaction indexing
    TxIndexFinishedBlocks  uint64 // Blocks with indexed transactions
    TxIndexRemainingBlocks uint64 // Blocks awaiting transaction indexing
}

// Done indicates whether initial synchronization is complete.
func (p SyncProgress) Done() bool {
    return p.CurrentBlock >= p.HighestBlock && p.TxIndexRemainingBlocks == 0
}

// ChainSyncReader provides access to the node's current synchronization status.
// Returns nil if no sync is in progress.
type ChainSyncReader interface {
    SyncProgress(ctx context.Context) (*SyncProgress, error)
}

// CallMsg defines parameters for contract calls.
type CallMsg struct {
    From          common.Address  // Transaction sender
    To            *common.Address // Target contract (nil for creation)
    Gas           uint64          // Gas limit (0 for near-infinite)
    GasPrice      *big.Int        // Legacy gas price
    GasFeeCap     *big.Int        // EIP-1559 max fee per gas
    GasTipCap     *big.Int        // EIP-1559 priority fee per gas
    Value         *big.Int        // Wei amount to send
    Data          []byte          // Input data (typically ABI-encoded)
    AccessList    types.AccessList // EIP-2930 access list
    BlobGasFeeCap *big.Int         // Blob transaction fee cap
    BlobHashes    []common.Hash    // Blob transaction hashes
}

// ContractCaller enables contract calls executed by the EVM without mining.
// For typed interactions, use abigen-generated bindings instead.
type ContractCaller interface {
    CallContract(ctx context.Context, call CallMsg, blockNumber *big.Int) ([]byte, error)
}

// FilterQuery specifies criteria for contract log filtering.
type FilterQuery struct {
    BlockHash *common.Hash     // Filter by specific block hash
    FromBlock *big.Int         // Start of range (nil for genesis)
    ToBlock   *big.Int         // End of range (nil for latest)
    Addresses []common.Address // Filter by contract addresses
    Topics    [][]common.Hash  // Filter by event topics
}

// LogFilterer provides access to contract logs via queries or subscriptions.
// Subscription logs may be marked Removed if reverted by reorganization.
type LogFilterer interface {
    FilterLogs(ctx context.Context, q FilterQuery) ([]types.Log, error)
    SubscribeFilterLogs(ctx context.Context, q FilterQuery, ch chan<- types.Log) (Subscription, error)
}

// TransactionSender handles sending signed transactions to the network.
// Use TransactionReceipt to get contract addresses after mining creations.
type TransactionSender interface {
    SendTransaction(ctx context.Context, tx *types.Transaction) error
}

// GasPricer suggests optimal gas prices based on current network conditions.
type GasPricer interface {
    SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

// GasPricer1559 provides EIP-1559 gas pricing suggestions.
type GasPricer1559 interface {
    SuggestGasTipCap(ctx context.Context) (*big.Int, error)
}

// FeeHistory contains historical fee market data for gas price estimation.
type FeeHistory struct {
    OldestBlock  *big.Int     // First block in the history
    Reward       [][]*big.Int // Priority fees per block
    BaseFee      []*big.Int   // Base fees per block
    GasUsedRatio []float64    // Gas usage ratios
}

// FeeHistoryReader provides access to historical fee data.
type FeeHistoryReader interface {
    FeeHistory(ctx context.Context, blockCount uint64, lastBlock *big.Int, rewardPercentiles []float64) (*FeeHistory, error)
}

// PendingStateReader accesses the pending state before blockchain inclusion.
type PendingStateReader interface {
    PendingBalanceAt(ctx context.Context, account common.Address) (*big.Int, error)
    PendingStorageAt(ctx context.Context, account common.Address, key common.Hash) ([]byte, error)
    PendingCodeAt(ctx context.Context, account common.Address) ([]byte, error)
    PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
    PendingTransactionCount(ctx context.Context) (uint, error)
}

// PendingContractCaller performs calls against the pending state.
type PendingContractCaller interface {
    PendingCallContract(ctx context.Context, call CallMsg) ([]byte, error)
}

// GasEstimator estimates gas requirements for transactions.
type GasEstimator interface {
    EstimateGas(ctx context.Context, call CallMsg) (uint64, error)
}

// PendingStateEventer provides real-time pending state change notifications.
type PendingStateEventer interface {
    SubscribePendingTransactions(ctx context.Context, ch chan<- *types.Transaction) (Subscription, error)
}

// BlockNumberReader retrieves the current block number.
type BlockNumberReader interface {
    BlockNumber(ctx context.Context) (uint64, error)
}

// ChainIDReader retrieves the blockchain's chain ID.
type ChainIDReader interface {
    ChainID(ctx context.Context) (*big.Int, error)
}
