package wallet

import (
	"math"

	"github.com/rivine/rivine/build"
	"github.com/rivine/rivine/modules"
	"github.com/rivine/rivine/types"
)

// updateConfirmedSet uses a consensus change to update the confirmed set of
// outputs as understood by the wallet.
func (w *Wallet) updateConfirmedSet(cc modules.ConsensusChange) {
	for _, diff := range cc.CoinOutputDiffs {
		// Verify that the diff is relevant to the wallet.
		_, exists := w.keys[diff.CoinOutput.UnlockHash]
		if !exists {
			continue
		}

		_, exists = w.coinOutputs[diff.ID]
		if diff.Direction == modules.DiffApply {
			if build.DEBUG && exists {
				panic("adding an existing output to wallet")
			}
			w.coinOutputs[diff.ID] = diff.CoinOutput
		} else {
			if build.DEBUG && !exists {
				panic("deleting nonexisting output from wallet")
			}
			delete(w.coinOutputs, diff.ID)
		}
	}
	for _, diff := range cc.BlockStakeOutputDiffs {
		// Verify that the diff is relevant to the wallet.
		_, exists := w.keys[diff.BlockStakeOutput.UnlockHash]
		if !exists {
			continue
		}

		_, exists = w.blockstakeOutputs[diff.ID]
		if diff.Direction == modules.DiffApply {
			if build.DEBUG && exists {
				panic("adding an existing output to wallet")
			}
			w.blockstakeOutputs[diff.ID] = diff.BlockStakeOutput
		} else {
			if build.DEBUG && !exists {
				panic("deleting nonexisting output from wallet")
			}
			delete(w.blockstakeOutputs, diff.ID)
		}
	}
}

// revertHistory reverts any transaction history that was destroyed by reverted
// blocks in the consensus change.
func (w *Wallet) revertHistory(cc modules.ConsensusChange) {
	for _, block := range cc.RevertedBlocks {
		// Remove any transactions that have been reverted.
		for i := len(block.Transactions) - 1; i >= 0; i-- {
			// If the transaction is relevant to the wallet, it will be the
			// most recent transaction appended to w.processedTransactions.
			// Relevance can be determined just by looking at the last element
			// of w.processedTransactions.
			txn := block.Transactions[i]
			txid := txn.ID()
			if len(w.processedTransactions) > 0 && txid == w.processedTransactions[len(w.processedTransactions)-1].TransactionID {
				w.processedTransactions = w.processedTransactions[:len(w.processedTransactions)-1]
				delete(w.processedTransactionMap, txid)
			}
		}

		// Remove the miner payout transaction if applicable.
		for _, mp := range block.MinerPayouts {
			_, exists := w.keys[mp.UnlockHash]
			if exists {
				w.processedTransactions = w.processedTransactions[:len(w.processedTransactions)-1]
				delete(w.processedTransactionMap, types.TransactionID(block.ID()))
				break
			}
		}
		w.consensusSetHeight--
	}
}

// applyHistory applies any transaction history that was introduced by the
// applied blocks.
func (w *Wallet) applyHistory(cc modules.ConsensusChange) {
	for _, block := range cc.AppliedBlocks {
		w.consensusSetHeight++
		// Apply the miner payout transaction if applicable.
		minerPT := modules.ProcessedTransaction{
			Transaction:           types.Transaction{},
			TransactionID:         types.TransactionID(block.ID()),
			ConfirmationHeight:    w.consensusSetHeight,
			ConfirmationTimestamp: block.Timestamp,
		}
		relevant := false
		for i, mp := range block.MinerPayouts {
			_, exists := w.keys[mp.UnlockHash]
			if exists {
				relevant = true
			}
			minerPT.Outputs = append(minerPT.Outputs, modules.ProcessedOutput{
				FundType:       types.SpecifierMinerPayout,
				MaturityHeight: w.consensusSetHeight + types.MaturityDelay,
				WalletAddress:  exists,
				RelatedAddress: mp.UnlockHash,
				Value:          mp.Value,
			})
			w.historicOutputs[types.OutputID(block.MinerPayoutID(uint64(i)))] = mp.Value
		}
		if relevant {
			w.processedTransactions = append(w.processedTransactions, minerPT)
			w.processedTransactionMap[minerPT.TransactionID] = &w.processedTransactions[len(w.processedTransactions)-1]
		}
		for ti, txn := range block.Transactions {
			relevant := false
			pt := modules.ProcessedTransaction{
				Transaction:           txn,
				TransactionID:         txn.ID(),
				ConfirmationHeight:    w.consensusSetHeight,
				ConfirmationTimestamp: block.Timestamp,
			}
			for _, sci := range txn.CoinInputs {
				_, exists := w.keys[sci.UnlockConditions.UnlockHash()]
				if exists {
					relevant = true
				}
				pt.Inputs = append(pt.Inputs, modules.ProcessedInput{
					FundType:       types.SpecifierCoinInput,
					WalletAddress:  exists,
					RelatedAddress: sci.UnlockConditions.UnlockHash(),
					Value:          w.historicOutputs[types.OutputID(sci.ParentID)],
				})
			}
			for i, sco := range txn.CoinOutputs {
				_, exists := w.keys[sco.UnlockHash]
				if exists {
					relevant = true
				}
				pt.Outputs = append(pt.Outputs, modules.ProcessedOutput{
					FundType:       types.SpecifierCoinOutput,
					MaturityHeight: w.consensusSetHeight,
					WalletAddress:  exists,
					RelatedAddress: sco.UnlockHash,
					Value:          sco.Value,
				})
				w.historicOutputs[types.OutputID(txn.CoinOutputID(uint64(i)))] = sco.Value
			}
			for _, sfi := range txn.BlockStakeInputs {
				_, exists := w.keys[sfi.UnlockConditions.UnlockHash()]
				if exists {
					relevant = true
				}
				sfiValue := w.historicOutputs[types.OutputID(sfi.ParentID)]
				pt.Inputs = append(pt.Inputs, modules.ProcessedInput{
					FundType:       types.SpecifierBlockStakeInput,
					WalletAddress:  exists,
					RelatedAddress: sfi.UnlockConditions.UnlockHash(),
					Value:          sfiValue,
				})
			}
			for i, sfo := range txn.BlockStakeOutputs {
				_, exists := w.keys[sfo.UnlockHash]
				if exists {
					relevant = true
				}
				pt.Outputs = append(pt.Outputs, modules.ProcessedOutput{
					FundType:       types.SpecifierBlockStakeOutput,
					MaturityHeight: w.consensusSetHeight,
					WalletAddress:  exists,
					RelatedAddress: sfo.UnlockHash,
					Value:          sfo.Value,
				})
				bsoid := txn.BlockStakeOutputID(uint64(i))
				_, exists = w.blockstakeOutputs[bsoid]
				if exists {
					w.unspentblockstakeoutputs[bsoid] = types.UnspentBlockStakeOutput{
						BlockHeight:      w.consensusSetHeight,
						TransactionIndex: uint64(ti),
						OutputIndex:      uint64(i),
						Value:            sfo.Value,
					}
				}
				w.historicOutputs[types.OutputID(bsoid)] = sfo.Value
			}
			for _, fee := range txn.MinerFees {
				pt.Outputs = append(pt.Outputs, modules.ProcessedOutput{
					FundType: types.SpecifierMinerFee,
					Value:    fee,
				})
			}
			if relevant {
				w.processedTransactions = append(w.processedTransactions, pt)
				w.processedTransactionMap[pt.TransactionID] = &w.processedTransactions[len(w.processedTransactions)-1]
			}
		}
	}
}

// ProcessConsensusChange parses a consensus change to update the set of
// confirmed outputs known to the wallet.
func (w *Wallet) ProcessConsensusChange(cc modules.ConsensusChange) {
	if err := w.tg.Add(); err != nil {
		// The wallet should gracefully reject updates from the consensus set
		// or transaction pool that are sent after the wallet's Close method
		// has closed the wallet's ThreadGroup.
		return
	}
	defer w.tg.Done()
	w.mu.Lock()
	defer w.mu.Unlock()
	w.updateConfirmedSet(cc)
	w.revertHistory(cc)
	w.applyHistory(cc)
}

// ReceiveUpdatedUnconfirmedTransactions updates the wallet's unconfirmed
// transaction set.
func (w *Wallet) ReceiveUpdatedUnconfirmedTransactions(txns []types.Transaction, _ modules.ConsensusChange) {
	if err := w.tg.Add(); err != nil {
		// Gracefully reject transactions if the wallet's Close method has
		// closed the wallet's ThreadGroup already.
		return
	}
	defer w.tg.Done()
	w.mu.Lock()
	defer w.mu.Unlock()

	w.unconfirmedProcessedTransactions = nil
	for _, txn := range txns {
		// To save on code complexity, relevancy is determined while building
		// up the wallet transaction.
		relevant := false
		pt := modules.ProcessedTransaction{
			Transaction:           txn,
			TransactionID:         txn.ID(),
			ConfirmationHeight:    types.BlockHeight(math.MaxUint64),
			ConfirmationTimestamp: types.Timestamp(math.MaxUint64),
		}
		for _, sci := range txn.CoinInputs {
			_, exists := w.keys[sci.UnlockConditions.UnlockHash()]
			if exists {
				relevant = true
			}
			pt.Inputs = append(pt.Inputs, modules.ProcessedInput{
				FundType:       types.SpecifierCoinInput,
				WalletAddress:  exists,
				RelatedAddress: sci.UnlockConditions.UnlockHash(),
				Value:          w.historicOutputs[types.OutputID(sci.ParentID)],
			})
		}
		for i, sco := range txn.CoinOutputs {
			_, exists := w.keys[sco.UnlockHash]
			if exists {
				relevant = true
			}
			pt.Outputs = append(pt.Outputs, modules.ProcessedOutput{
				FundType:       types.SpecifierCoinOutput,
				MaturityHeight: types.BlockHeight(math.MaxUint64),
				WalletAddress:  exists,
				RelatedAddress: sco.UnlockHash,
				Value:          sco.Value,
			})
			w.historicOutputs[types.OutputID(txn.CoinOutputID(uint64(i)))] = sco.Value
		}
		for _, fee := range txn.MinerFees {
			pt.Outputs = append(pt.Outputs, modules.ProcessedOutput{
				FundType: types.SpecifierMinerFee,
				Value:    fee,
			})
		}
		if relevant {
			w.unconfirmedProcessedTransactions = append(w.unconfirmedProcessedTransactions, pt)
		}
	}
}
