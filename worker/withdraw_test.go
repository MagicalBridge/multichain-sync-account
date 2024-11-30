package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/dapplink-labs/multichain-sync-account/config"
	"github.com/dapplink-labs/multichain-sync-account/database"
	"github.com/dapplink-labs/multichain-sync-account/rpcclient"
	"github.com/dapplink-labs/multichain-sync-account/rpcclient/chain-account/account"
)

const (
	businessId = "1"
	txId       = "f42ebac2-1b23-45d1-bac1-f1dda168a1c4"
)

func setupDb() *database.DB {
	dbConfig := config.DBConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		Name:     "postgres",
		User:     "louis",
		Password: "cpf1234",
	}

	newDB, _ := database.NewDB(context.Background(), dbConfig)
	return newDB
}

func setupWithdraw(t *testing.T) *Withdraw {
	// 设置数据库
	db := setupDb()

	// 设置 gRPC 连接
	conn, err := grpc.NewClient("127.0.0.1:8189", grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)

	// 设置账户客户端
	client := account.NewWalletAccountServiceClient(conn)
	accountClient, err := rpcclient.NewWalletChainAccountClient(context.Background(), client, "Ethereum")
	assert.NoError(t, err)

	// 设置配置
	cfg := &config.Config{
		ChainNode: config.ChainNodeConfig{
			WorkerInterval: 5 * time.Second,
		},
	}

	// 创建 shutdown 函数
	shutdown := func(cause error) {
		t.Logf("Shutdown called with cause: %v", cause)
	}

	// 创建 Withdraw worker
	withdraw, err := NewWithdraw(cfg, db, accountClient, shutdown)
	assert.NoError(t, err)

	return withdraw
}

func TestWithdraw_Start(t *testing.T) {
	// 准备测试环境
	withdraw := setupWithdraw(t)

	// 启动 worker
	err := withdraw.Start()
	assert.NoError(t, err)

	// 等待一段时间让 worker 处理交易
	time.Sleep(1000 * time.Second)

	// 验证交易是否被处理
	processedTx, err := withdraw.db.Withdraws.QueryWithdrawsByHash(businessId, txId)
	assert.NoError(t, err)
	assert.NotNil(t, processedTx)

	// 验证状态更新
	//assert.Equal(t, uint8(2), processedTx.Status)

	// 清理资源
	err = withdraw.Close()
	assert.NoError(t, err)
}
