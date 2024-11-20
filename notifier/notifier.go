package notifier

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"

	"github.com/dapplink-labs/multichain-sync-account/common/tasks"
	"github.com/dapplink-labs/multichain-sync-account/database"
)

type Notifier struct {
	db             *database.DB
	businessIds    []string
	notifyClient   map[string]*NotifyClient
	resourceCtx    context.Context
	resourceCancel context.CancelFunc
	tasks          tasks.Group
	ticker         *time.Ticker
}

func NewNotifier(db *database.DB, shutdown context.CancelCauseFunc) (*Notifier, error) {
	businessList, err := db.Business.QueryBusinessList()
	if err != nil {
		log.Error("query business list fail", "err", err)
		return nil, err
	}

	var businessIds []string
	var notifyClient map[string]*NotifyClient
	for _, business := range businessList {
		businessIds = append(businessIds, business.BusinessUid)
		client, err := NewNotifierClient(business.NotifyUrl)
		if err != nil {
			log.Error("new notify client fail", "err", err)
			return nil, err
		}
		notifyClient[business.BusinessUid] = client
	}

	resCtx, resCancel := context.WithCancel(context.Background())
	return &Notifier{
		db:             db,
		notifyClient:   notifyClient,
		businessIds:    businessIds,
		resourceCtx:    resCtx,
		resourceCancel: resCancel,
		tasks: tasks.Group{HandleCrit: func(err error) {
			shutdown(fmt.Errorf("critical error in internals: %w", err))
		}},
		ticker: time.NewTicker(time.Second * 5),
	}, nil
}

func (nf *Notifier) Close() error {
	var result error
	nf.resourceCancel()
	nf.ticker.Stop()
	if err := nf.tasks.Wait(); err != nil {
		result = errors.Join(result, fmt.Errorf("failed to await notify %w"), err)
		return result
	}
	log.Info("stop notify success")
	return nil
}

func (nf *Notifier) Start() error {
	log.Info("start internals......")
	nf.tasks.Go(func() error {
		for {
			select {
			case <-nf.ticker.C:
				var txn []Transaction
				for _, businessId := range nf.businessIds {
					log.Info("txn and businessId", "txn", txn, "businessId", businessId)
					// 获取需要通知充值
					// 获取需要通知提现
					// 获取需要通知归集
					// 修改通知状态
					// 根据业务通知上层业务
					// 修改本地数据状态
				}
			case <-nf.resourceCtx.Done():
				log.Info("stop internals in worker")
				return nil
			}
		}
	})
	return nil
}
