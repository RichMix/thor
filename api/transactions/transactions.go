package transactions

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/vechain/thor/api/utils"
	"github.com/vechain/thor/chain"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"
	"github.com/vechain/thor/txpool"
)

type Transactions struct {
	chain *chain.Chain
	pool  *txpool.TxPool
}

func New(chain *chain.Chain, pool *txpool.TxPool) *Transactions {
	return &Transactions{
		chain,
		pool,
	}
}

func (t *Transactions) getTransactionByID(txID thor.Bytes32) (*Transaction, error) {
	if pengdingTransaction := t.pool.GetTransaction(txID); pengdingTransaction != nil {
		return ConvertTransaction(pengdingTransaction)
	}
	tx, location, err := t.chain.GetTransaction(txID)
	if err != nil {
		if t.chain.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	block, err := t.chain.GetBlock(location.BlockID)
	if err != nil {
		if t.chain.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	tc, err := ConvertTransaction(tx)
	if err != nil {
		return nil, err
	}
	tc.Block.ID = block.Header().ID()
	tc.Block.Number = block.Header().Number()
	tc.Block.Timestamp = block.Header().Timestamp()
	return tc, nil
}

//GetTransactionReceiptByID get tx's receipt
func (t *Transactions) getTransactionReceiptByID(txID thor.Bytes32) (*Receipt, error) {
	tx, location, err := t.chain.GetTransaction(txID)
	if err != nil {
		if t.chain.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	block, err := t.chain.GetBlock(location.BlockID)
	if err != nil {
		if t.chain.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	receipts, err := t.chain.GetBlockReceipts(block.Header().ID())
	if err != nil {
		if t.chain.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	rece := receipts[location.Index]
	return convertReceipt(rece, block, tx)
}

//SendRawTransaction send a raw transactoion
func (t *Transactions) sendRawTransaction(rawTx *RawTx) (*thor.Bytes32, error) {
	data, err := hexutil.Decode(rawTx.Raw)
	if err != nil {
		return nil, err
	}
	var tx *tx.Transaction
	if err := rlp.DecodeBytes(data, &tx); err != nil {
		return nil, err
	}
	if err := t.pool.Add(tx); err != nil {
		return nil, err
	}
	txID := tx.ID()
	return &txID, nil
}

func (t *Transactions) handleSendTransaction(w http.ResponseWriter, req *http.Request) error {
	res, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return utils.HTTPError(err, http.StatusBadRequest)
	}
	req.Body.Close()
	if len(res) == 0 {
		return utils.HTTPError(errors.New("transaction required"), http.StatusBadRequest)
	}
	var raw *RawTx
	if err = json.Unmarshal(res, &raw); err != nil {
		return utils.HTTPError(err, http.StatusBadRequest)
	}
	txID, err := t.sendRawTransaction(raw)
	if err != nil {
		return utils.HTTPError(err, http.StatusBadRequest)
	}
	return utils.WriteJSON(w, map[string]string{
		"id": txID.String(),
	})
}

func (t *Transactions) handleGetTransactionByID(w http.ResponseWriter, req *http.Request) error {
	id := mux.Vars(req)["id"]
	txID, err := thor.ParseBytes32(id)
	if err != nil {
		return utils.HTTPError(errors.Wrap(err, "id"), http.StatusBadRequest)
	}
	tx, err := t.getTransactionByID(txID)
	if err != nil {
		return utils.HTTPError(err, http.StatusBadRequest)
	}
	return utils.WriteJSON(w, tx)
}

func (t *Transactions) handleGetTransactionReceiptByID(w http.ResponseWriter, req *http.Request) error {
	id := mux.Vars(req)["id"]
	txID, err := thor.ParseBytes32(id)
	if err != nil {
		return utils.HTTPError(errors.Wrap(err, "id"), http.StatusBadRequest)
	}
	receipt, err := t.getTransactionReceiptByID(txID)
	if err != nil {
		return utils.HTTPError(err, http.StatusBadRequest)
	}
	return utils.WriteJSON(w, receipt)
}

func (t *Transactions) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(t.handleSendTransaction))
	sub.Path("/{id}").Methods("GET").HandlerFunc(utils.WrapHandlerFunc(t.handleGetTransactionByID))
	sub.Path("/{id}/receipt").Methods("GET").HandlerFunc(utils.WrapHandlerFunc(t.handleGetTransactionReceiptByID))
}
