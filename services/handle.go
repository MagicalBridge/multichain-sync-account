package services

import (
	"context"
	"encoding/base64"
	"math/big"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/dapplink-labs/multichain-sync-account/common/json2"
	"github.com/dapplink-labs/multichain-sync-account/database"
	"github.com/dapplink-labs/multichain-sync-account/database/dynamic"
	dal_wallet_go "github.com/dapplink-labs/multichain-sync-account/protobuf/dal-wallet-go"
	"github.com/dapplink-labs/multichain-sync-account/rpcclient/chain-account/account"
)

const (
	ChainName = "Ethereum"
	Network   = "mainnet"
)

var (
	EthGasLimit          uint64 = 60000
	TokenGasLimit        uint64 = 120000
	maxFeePerGas                = "13517748000"
	maxPriorityFeePerGas        = "53517748000"
)

func (bws *BusinessMiddleWireServices) BusinessRegister(ctx context.Context, request *dal_wallet_go.BusinessRegisterRequest) (*dal_wallet_go.BusinessRegisterResponse, error) {
	if request.RequestId == "" || request.NotifyUrl == "" {
		return &dal_wallet_go.BusinessRegisterResponse{
			Code: dal_wallet_go.ReturnCode_ERROR,
			Msg:  "invalid params",
		}, nil
	}
	business := &database.Business{
		GUID:        uuid.New(),
		BusinessUid: request.RequestId,
		NotifyUrl:   request.NotifyUrl,
		Timestamp:   uint64(time.Now().Unix()),
	}
	err := bws.db.Business.StoreBusiness(business)
	if err != nil {
		log.Error("store business fail", "err", err)
		return &dal_wallet_go.BusinessRegisterResponse{
			Code: dal_wallet_go.ReturnCode_ERROR,
			Msg:  "store db fail",
		}, nil
	}
	dynamic.CreateTableFromTemplate(request.RequestId, bws.db)
	return &dal_wallet_go.BusinessRegisterResponse{
		Code: dal_wallet_go.ReturnCode_SUCCESS,
		Msg:  "config business success",
	}, nil
}

func (bws *BusinessMiddleWireServices) ExportAddressesByPublicKeys(ctx context.Context, request *dal_wallet_go.ExportAddressesRequest) (*dal_wallet_go.ExportAddressesResponse, error) {
	var (
		retAddressess []*dal_wallet_go.Address
		dbAddresses   []database.Addresses
		balances      []database.Balances
	)

	for _, value := range request.PublicKeys {
		address := bws.accountClient.ExportAddressByPubKey(strconv.Itoa(int(value.Type)), value.PublicKey)
		item := &dal_wallet_go.Address{
			Type:    value.Type,
			Address: address,
		}
		dbAddress := database.Addresses{
			GUID:        uuid.New(),
			Address:     common.HexToAddress(address),
			AddressType: uint8(value.Type),
			PublicKey:   value.PublicKey,
			Timestamp:   uint64(time.Now().Unix()),
		}
		dbAddresses = append(dbAddresses, dbAddress)

		balanceItem := database.Balances{
			GUID:         uuid.New(),
			Address:      common.HexToAddress(address),
			TokenAddress: common.Address{},
			AddressType:  uint8(value.Type),
			Balance:      big.NewInt(0),
			LockBalance:  big.NewInt(0),
			Timestamp:    uint64(time.Now().Unix()),
		}
		balances = append(balances, balanceItem)

		retAddressess = append(retAddressess, item)
	}
	err := bws.db.Addresses.StoreAddresses(request.RequestId, dbAddresses)
	if err != nil {
		return &dal_wallet_go.ExportAddressesResponse{
			Code: dal_wallet_go.ReturnCode_ERROR,
			Msg:  "store address to db fail",
		}, nil
	}
	err = bws.db.Balances.StoreBalances(request.RequestId, balances)
	if err != nil {
		return &dal_wallet_go.ExportAddressesResponse{
			Code: dal_wallet_go.ReturnCode_ERROR,
			Msg:  "store balance to db fail",
		}, nil
	}
	return &dal_wallet_go.ExportAddressesResponse{
		Code:      dal_wallet_go.ReturnCode_SUCCESS,
		Msg:       "generate address success",
		Addresses: retAddressess,
	}, nil
}

func (bws *BusinessMiddleWireServices) CreateUnSignTransaction(ctx context.Context, request *dal_wallet_go.UnSignWithdrawTransactionRequest) (*dal_wallet_go.UnSignWithdrawTransactionResponse, error) {
	amountBig, _ := new(big.Int).SetString(request.Value, 10)
	transactionId := uuid.New()
	if request.TxType == "withdraw" {
		withdraw := &database.Withdraws{
			GUID:         transactionId,
			BlockHash:    common.Hash{},
			BlockNumber:  big.NewInt(1),
			Hash:         common.Hash{},
			FromAddress:  common.HexToAddress(request.From),
			ToAddress:    common.HexToAddress(request.To),
			TokenAddress: common.HexToAddress(request.ContractAddress),
			TokenId:      request.TokenId,
			TokenMeta:    request.TokenMeta,
			Fee:          big.NewInt(0),
			Amount:       amountBig,
			Status:       0,
			TxSignHex:    "",
			Timestamp:    uint64(time.Now().Unix()),
		}
		err := bws.db.Withdraws.StoreWithdraw(request.RequestId, withdraw)
		if err != nil {
			log.Error("store withdraw fail", "err", err)
			return nil, err
		}
	} else if request.TxType == "collection" || request.TxType == "hot2cold" {
		internal := &database.Internals{
			GUID:         transactionId,
			BlockHash:    common.Hash{},
			BlockNumber:  big.NewInt(1),
			Hash:         common.Hash{},
			FromAddress:  common.HexToAddress(request.From),
			ToAddress:    common.HexToAddress(request.To),
			TokenAddress: common.HexToAddress(request.ContractAddress),
			TokenId:      request.TokenId,
			TokenMeta:    request.TokenMeta,
			Fee:          big.NewInt(0),
			Amount:       amountBig,
			Status:       0,
			TxType:       request.TxType,
			TxSignHex:    "",
			Timestamp:    uint64(time.Now().Unix()),
		}
		err := bws.db.Internals.StoreInternal(request.RequestId, internal)
		if err != nil {
			log.Error("store internal business transaction fail", "err", err)
			return nil, err
		}
	} else {
		return &dal_wallet_go.UnSignWithdrawTransactionResponse{
			Code:          dal_wallet_go.ReturnCode_ERROR,
			Msg:           "Un support transaction type",
			TransactionId: transactionId.String(),
			UnSignTx:      "0x00",
		}, nil
	}

	accountReq := &account.AccountRequest{
		Chain:           ChainName,
		Network:         Network,
		Address:         request.From,
		ContractAddress: "0x00",
	}
	accountInfo, err := bws.accountClient.AccountRpClient.GetAccount(context.Background(), accountReq)
	if err != nil {
		log.Error("handle CreateUnSignTransaction GetAccount fail", "err", err)
		return nil, err
	}
	nonce, err := strconv.Atoi(accountInfo.Sequence)
	if err != nil {
		log.Error("handle CreateUnSignTransaction GetAccount strconv.Atoi(Sequence) fail", "err", err)
		return nil, err
	}
	var gasLimit uint64
	var contractAddress string
	if request.ContractAddress == "0x00" {
		contractAddress = "0x00"
		gasLimit = EthGasLimit
	} else {
		contractAddress = request.ContractAddress
		gasLimit = TokenGasLimit
	}
	txStructure := TxStructure{
		ChainId:         request.ChainId,
		Nonce:           uint64(nonce),
		GasPrice:        maxFeePerGas,
		GasTipCap:       maxFeePerGas,
		GasFeeCap:       maxPriorityFeePerGas,
		Gas:             gasLimit,
		ContractAddress: contractAddress,
		FromAddress:     request.From,
		ToAddress:       request.To,
		TokenId:         request.TokenId,
		Value:           request.Value,
	}
	data := json2.ToJSON(txStructure)
	base64Str := base64.StdEncoding.EncodeToString(data)

	unsignTx := &account.UnSignTransactionRequest{
		Chain:    ChainName,
		Network:  Network,
		Base64Tx: base64Str,
	}
	returnTx, err := bws.accountClient.AccountRpClient.CreateUnSignTransaction(context.Background(), unsignTx)
	log.Info("BusinessMiddleWireServices CreateUnSignTransaction txStructure", json2.ToJSONString(txStructure))
	if err != nil {
		log.Error("create un sign transaction fail", "err", err)
		return nil, err
	}
	return &dal_wallet_go.UnSignWithdrawTransactionResponse{
		Code:          dal_wallet_go.ReturnCode_SUCCESS,
		Msg:           "submit withdraw and build un sign tranaction success",
		TransactionId: transactionId.String(),
		UnSignTx:      returnTx.UnSignTx,
	}, nil
}

func (bws *BusinessMiddleWireServices) BuildSignedTransaction(ctx context.Context, request *dal_wallet_go.SignedWithdrawTransactionRequest) (*dal_wallet_go.SignedWithdrawTransactionResponse, error) {
	resp := &dal_wallet_go.SignedWithdrawTransactionResponse{
		Code: dal_wallet_go.ReturnCode_ERROR,
	}
	var txStructure TxStructure
	if request.TxType == "withdraw" {
		tx, err := bws.db.Withdraws.QueryWithdrawsByHash(request.RequestId, request.TransactionId)
		if err != nil {
			return nil, err
		}
		if tx == nil {
			resp.Msg = "TransactionId not found"
			return resp, nil
		}
		accountReq := &account.AccountRequest{
			Chain:           ChainName,
			Network:         Network,
			Address:         tx.FromAddress.String(),
			ContractAddress: "0x00",
		}
		accountInfo, err := bws.accountClient.AccountRpClient.GetAccount(context.Background(), accountReq)
		if err != nil {
			return nil, err
		}
		nonce, err := strconv.Atoi(accountInfo.Sequence)
		if err != nil {
			log.Error("handle BuildSignedTransaction GetAccount strconv.Atoi(Sequence) fail", "err", err)
			return nil, err
		}
		var gasLimit uint64
		var contractAddress string
		if tx.TokenAddress.String() == "0x00" || tx.TokenAddress.String() == "0x0000000000000000000000000000000000000000" {
			gasLimit = EthGasLimit
			contractAddress = "0x00"
		} else {
			gasLimit = TokenGasLimit
			contractAddress = tx.TokenAddress.String()
		}
		txStructure = TxStructure{
			ChainId:         request.ChainId,
			Nonce:           uint64(nonce),
			GasPrice:        maxFeePerGas,
			GasTipCap:       maxFeePerGas,
			GasFeeCap:       maxPriorityFeePerGas,
			Gas:             gasLimit,
			ContractAddress: contractAddress,
			FromAddress:     tx.FromAddress.String(),
			ToAddress:       tx.ToAddress.String(),
			TokenId:         tx.TokenId,
			Value:           tx.Amount.String(),
		}
	} else if request.TxType == "collection" || request.TxType == "hot2cold" {
		tx, err := bws.db.Internals.QueryInternalsByHash(request.RequestId, request.TransactionId)
		if err != nil {
			return nil, err
		}
		accountReq := &account.AccountRequest{
			Chain:           ChainName,
			Network:         Network,
			Address:         tx.FromAddress.String(),
			ContractAddress: "0x00",
		}
		accountInfo, err := bws.accountClient.AccountRpClient.GetAccount(context.Background(), accountReq)
		if err != nil {
			return nil, err
		}
		nonce, err := strconv.Atoi(accountInfo.Sequence)
		if err != nil {
			log.Error("handle BuildSignedTransaction GetAccount strconv.Atoi(Sequence) fail", "err", err)
			return nil, err
		}
		var gasLimit uint64
		var contractAddress string
		if tx.TokenAddress.String() == "0x00" || tx.TokenAddress.String() == "0x0000000000000000000000000000000000000000" {
			gasLimit = EthGasLimit
			contractAddress = "0x00"
		} else {
			gasLimit = TokenGasLimit
			contractAddress = tx.TokenAddress.String()
		}
		txStructure = TxStructure{
			ChainId:         request.ChainId,
			Nonce:           uint64(nonce),
			GasPrice:        maxFeePerGas,
			GasTipCap:       maxFeePerGas,
			GasFeeCap:       maxPriorityFeePerGas,
			Gas:             gasLimit,
			ContractAddress: contractAddress,
			FromAddress:     tx.FromAddress.String(),
			ToAddress:       tx.ToAddress.String(),
			TokenId:         tx.TokenId,
			Value:           tx.Amount.String(),
		}
	} else {
		return &dal_wallet_go.SignedWithdrawTransactionResponse{
			Code:     dal_wallet_go.ReturnCode_ERROR,
			Msg:      "Un support transaction type",
			SignedTx: "",
		}, nil
	}
	data := json2.ToJSON(txStructure)
	base64Str := base64.StdEncoding.EncodeToString(data)
	signedTx := &account.SignedTransactionRequest{
		Chain:     ChainName,
		Network:   Network,
		Signature: request.Signature,
		Base64Tx:  base64Str,
	}
	log.Info("BusinessMiddleWireServices BuildSignedTransaction txStructure", json2.ToJSONString(txStructure))
	returnTx, err := bws.accountClient.AccountRpClient.BuildSignedTransaction(context.Background(), signedTx)
	if err != nil {
		log.Error("create un sign transaction fail", "err", err)
		return nil, err
	}

	if request.TxType == "withdraw" {
		err = bws.db.Withdraws.UpdateWithdrawTx(request.RequestId, request.TransactionId, returnTx.SignedTx, nil, 1) // 1:交易已经签名
		if err != nil {
			log.Error("update signed tx to db fail", "err", err)
			return nil, err
		}
	} else {
		err = bws.db.Internals.UpdateInternalTx(request.RequestId, request.TransactionId, returnTx.SignedTx, nil, 1) // 1:交易已经签名
		if err != nil {
			log.Error("update signed tx to db fail", "err", err)
			return nil, err
		}
	}
	return &dal_wallet_go.SignedWithdrawTransactionResponse{
		Code:     1,
		Msg:      "build signed tx success",
		SignedTx: returnTx.SignedTx,
	}, nil
}

func (bws *BusinessMiddleWireServices) SetTokenAddress(ctx context.Context, request *dal_wallet_go.SetTokenAddressRequest) (*dal_wallet_go.SetTokenAddressResponse, error) {
	var (
		tokenList []database.Tokens
	)
	for _, value := range request.TokenList {
		CollectAmountBigInt, _ := new(big.Int).SetString(value.CollectAmount, 10)
		ColdAmountBigInt, _ := new(big.Int).SetString(value.ColdAmount, 10)
		token := database.Tokens{
			GUID:          uuid.New(),
			TokenAddress:  common.HexToAddress(value.Address),
			Decimals:      uint8(value.Decimals),
			TokenName:     value.TokenName,
			CollectAmount: CollectAmountBigInt,
			ColdAmount:    ColdAmountBigInt,
			Timestamp:     uint64(time.Now().Unix()),
		}
		tokenList = append(tokenList, token)
	}
	err := bws.db.Tokens.StoreTokens(request.RequestId, tokenList)
	if err != nil {
		log.Error("set token address fail", "err", err)
		return nil, err
	}
	return &dal_wallet_go.SetTokenAddressResponse{
		Code: 1,
		Msg:  "set token address success",
	}, nil
}
