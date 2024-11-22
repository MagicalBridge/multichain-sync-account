package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/dapplink-labs/multichain-sync-account/common/retry"
	"github.com/dapplink-labs/multichain-sync-account/common/tasks"
	"github.com/dapplink-labs/multichain-sync-account/config"
	"github.com/dapplink-labs/multichain-sync-account/database"
	"github.com/dapplink-labs/multichain-sync-account/rpcclient"
)

type Withdraw struct {
	rpcClient      *rpcclient.WalletChainAccountClient
	db             *database.DB
	chainNodeConf  *config.ChainNodeConfig
	resourceCtx    context.Context
	resourceCancel context.CancelFunc
	tasks          tasks.Group
	ticker         *time.Ticker
}

func NewWithdraw(cfg *config.Config, db *database.DB, shutdown context.CancelCauseFunc) (*Withdraw, error) {
	resCtx, resCancel := context.WithCancel(context.Background())
	return &Withdraw{
		db:             db,
		chainNodeConf:  &cfg.ChainNode,
		resourceCtx:    resCtx,
		resourceCancel: resCancel,
		tasks: tasks.Group{HandleCrit: func(err error) {
			shutdown(fmt.Errorf("critical error in withdraw: %w", err))
		}},
		ticker: time.NewTicker(time.Second * 5),
	}, nil
}

func (w *Withdraw) Close() error {
	var result error
	w.resourceCancel()
	w.ticker.Stop()
	log.Info("stop withdraw......")
	if err := w.tasks.Wait(); err != nil {
		result = errors.Join(result, fmt.Errorf("failed to await withdraw %w"), err)
		return result
	}
	log.Info("stop withdraw success")
	return nil
}

func (w *Withdraw) Start() error {
	log.Info("start withdraw......")
	w.tasks.Go(func() error {
		for {
			select {
			case <-w.ticker.C:
				businessList, err := w.db.Business.QueryBusinessList()
				if err != nil {
					log.Error("query business list fail", "err", err)
					return err
				}

				for _, businessId := range businessList {
					unSendTransactionList, err := w.db.Withdraws.UnSendWithdrawsList(businessId.BusinessUid)
					if err != nil {
						return err
					}

					var balanceList []database.Balances

					for _, unSendTransaction := range unSendTransactionList {
						balanceItem := database.Balances{
							TokenAddress: unSendTransaction.TokenAddress,
							Address:      unSendTransaction.FromAddress,
							LockBalance:  unSendTransaction.Amount,
						}
						balanceList = append(balanceList, balanceItem)
						txHash, err := w.rpcClient.SendTx(unSendTransaction.TxSignHex)
						if err != nil {
							log.Error("send transaction fail", "err", err)
							return err
						} else {
							unSendTransaction.Hash = common.HexToHash(txHash)
							unSendTransaction.Status = 2
						}
					}

					retryStrategy := &retry.ExponentialStrategy{Min: 1000, Max: 20_000, MaxJitter: 250}
					if _, err := retry.Do[interface{}](w.resourceCtx, 10, retryStrategy, func() (interface{}, error) {
						if err := w.db.Transaction(func(tx *database.DB) error {
							if len(balanceList) > 0 {
								log.Info("Update address balance", "totalTx", len(balanceList))
								if err := tx.Balances.UpdateBalances(businessId.BusinessUid, balanceList); err != nil {
									log.Error("Update address balance fail", "err", err)
									return err
								}

							}
							if len(unSendTransactionList) > 0 {
								err = w.db.Withdraws.UpdateWithdrawStatus(businessId.BusinessUid, 2, unSendTransactionList)
								if err != nil {
									log.Error("update withdraw status fail", "err", err)
									return err
								}
							}
							return nil
						}); err != nil {
							log.Error("unable to persist batch", "err", err)
							return nil, err
						}
						return nil, nil
					}); err != nil {
						return err
					}
				}

			case <-w.resourceCtx.Done():
				log.Info("stop withdraw in worker")
				return nil
			}
		}
	})
	return nil
}