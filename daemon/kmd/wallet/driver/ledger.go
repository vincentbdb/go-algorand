// Copyright (C) 2019 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package driver

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/algorand/go-deadlock"

	"github.com/vincentbdb/go-algorand/crypto"
	"github.com/vincentbdb/go-algorand/daemon/kmd/config"
	"github.com/vincentbdb/go-algorand/daemon/kmd/wallet"
	"github.com/vincentbdb/go-algorand/data/transactions"
	"github.com/vincentbdb/go-algorand/protocol"
)

const (
	ledgerWalletDriverName    = "ledger"
	ledgerWalletDriverVersion = 1

	ledgerClass            = uint8(0x80)
	ledgerInsGetPublicKey  = uint8(0x03)
	ledgerInsSignPaymentV2 = uint8(0x04)
	ledgerInsSignKeyregV2  = uint8(0x05)
	ledgerInsSignMsgpack   = uint8(0x08)
	ledgerP1first          = uint8(0x00)
	ledgerP1more           = uint8(0x80)
	ledgerP2last           = uint8(0x00)
	ledgerP2more           = uint8(0x80)
)

var ledgerWalletSupportedTxs = []protocol.TxType{protocol.PaymentTx, protocol.KeyRegistrationTx}

// LedgerWalletDriver provides access to a hardware wallet on the
// Ledger Nano S device.  The device must run the Algorand wallet
// application from https://github.com/algorand/ledger-app-algorand
type LedgerWalletDriver struct {
	wallets map[string]*LedgerWallet
}

// LedgerWallet represents a particular wallet under the
// LedgerWalletDriver.  The lock prevents concurrent access
// to the USB device.
type LedgerWallet struct {
	mu  deadlock.Mutex
	dev LedgerUSB
}

// CreateWallet implements the Driver interface.  There is
// currently no way to create new wallet keys; there is one
// key in a hardware wallet, derived from the device master
// secret.  We could, in principle, derive multiple keys.
// This is not supported at the moment.
func (lwd *LedgerWalletDriver) CreateWallet(name []byte, id []byte, pw []byte, mdk crypto.MasterDerivationKey) error {
	return errNotSupported
}

// FetchWallet looks up a wallet by ID and returns it, failing if there's more
// than one wallet with the given ID
func (lwd *LedgerWalletDriver) FetchWallet(id []byte) (w wallet.Wallet, err error) {
	lw, ok := lwd.wallets[string(id)]
	if !ok {
		return nil, errWalletNotFound
	}

	return lw, nil
}

// InitWithConfig accepts a driver configuration.  Currently, the Ledger
// driver does not have any configuration parameters.  However, we use
// this to enumerate the USB devices.
func (lwd *LedgerWalletDriver) InitWithConfig(cfg config.KMDConfig) error {
	devs, err := LedgerEnumerate()
	if err != nil {
		return err
	}

	lwd.wallets = make(map[string]*LedgerWallet)
	for _, dev := range devs {
		id := dev.USBInfo().Path
		lwd.wallets[id] = &LedgerWallet{
			dev: dev,
		}
	}
	return nil
}

// ListWalletMetadatas returns all wallets supported by this driver.
func (lwd *LedgerWalletDriver) ListWalletMetadatas() (metadatas []wallet.Metadata, err error) {
	for _, w := range lwd.wallets {
		md, err := w.Metadata()
		if err != nil {
			return nil, err
		}

		metadatas = append(metadatas, md)
	}

	return metadatas, nil
}

// RenameWallet implements the Driver interface.
func (lwd *LedgerWalletDriver) RenameWallet(newName []byte, id []byte, pw []byte) error {
	return errNotSupported
}

// Init implements the wallet interface.
func (lw *LedgerWallet) Init(pw []byte) error {
	return nil
}

// CheckPassword implements the Wallet interface.
func (lw *LedgerWallet) CheckPassword(pw []byte) error {
	return nil
}

// ExportMasterDerivationKey implements the Wallet interface.
func (lw *LedgerWallet) ExportMasterDerivationKey(pw []byte) (crypto.MasterDerivationKey, error) {
	return crypto.MasterDerivationKey{}, errNotSupported
}

// Metadata implements the Wallet interface.
func (lw *LedgerWallet) Metadata() (wallet.Metadata, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	info := lw.dev.USBInfo()
	return wallet.Metadata{
		ID:                    []byte(info.Path),
		Name:                  []byte(fmt.Sprintf("%s %s (serial %s, path %s)", info.Manufacturer, info.Product, info.Serial, info.Path)),
		DriverName:            ledgerWalletDriverName,
		DriverVersion:         ledgerWalletDriverVersion,
		SupportedTransactions: ledgerWalletSupportedTxs,
	}, nil
}

// ListKeys implements the Wallet interface.
func (lw *LedgerWallet) ListKeys() ([]crypto.Digest, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	reply, err := lw.dev.Exchange([]byte{ledgerClass, ledgerInsGetPublicKey, 0x00, 0x00, 0x00})
	if err != nil {
		return nil, err
	}

	var addr crypto.Digest
	copy(addr[:], reply)
	return []crypto.Digest{addr}, nil
}

// ImportKey implements the Wallet interface.
func (lw *LedgerWallet) ImportKey(sk crypto.PrivateKey) (crypto.Digest, error) {
	return crypto.Digest{}, errNotSupported
}

// ExportKey implements the Wallet interface.
func (lw *LedgerWallet) ExportKey(pk crypto.Digest, pw []byte) (crypto.PrivateKey, error) {
	return crypto.PrivateKey{}, errNotSupported
}

// GenerateKey implements the Wallet interface.
func (lw *LedgerWallet) GenerateKey(displayMnemonic bool) (crypto.Digest, error) {
	return crypto.Digest{}, errNotSupported
}

// DeleteKey implements the Wallet interface.
func (lw *LedgerWallet) DeleteKey(pk crypto.Digest, pw []byte) error {
	return errNotSupported
}

// ImportMultisigAddr implements the Wallet interface.
func (lw *LedgerWallet) ImportMultisigAddr(version, threshold uint8, pks []crypto.PublicKey) (crypto.Digest, error) {
	return crypto.Digest{}, errNotSupported
}

// LookupMultisigPreimage implements the Wallet interface.
func (lw *LedgerWallet) LookupMultisigPreimage(crypto.Digest) (version, threshold uint8, pks []crypto.PublicKey, err error) {
	return 0, 0, nil, errNotSupported
}

// ListMultisigAddrs implements the Wallet interface.
func (lw *LedgerWallet) ListMultisigAddrs() (addrs []crypto.Digest, err error) {
	return nil, nil
}

// DeleteMultisigAddr implements the Wallet interface.
func (lw *LedgerWallet) DeleteMultisigAddr(addr crypto.Digest, pw []byte) error {
	return errNotSupported
}

// SignTransaction implements the Wallet interface.
func (lw *LedgerWallet) SignTransaction(tx transactions.Transaction, pw []byte) ([]byte, error) {
	sig, err := lw.signTransactionHelper(tx)
	if err != nil {
		return nil, err
	}

	return protocol.Encode(transactions.SignedTxn{
		Txn: tx,
		Sig: sig,
	}), nil
}

// SignProgram implements the Wallet interface.
func (lw *LedgerWallet) SignProgram(data []byte, src crypto.Digest, pw []byte) ([]byte, error) {
	sig, err := lw.signProgramHelper(data)
	if err != nil {
		return nil, err
	}

	return sig[:], nil
}

// MultisigSignTransaction implements the Wallet interface.
func (lw *LedgerWallet) MultisigSignTransaction(tx transactions.Transaction, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte) (crypto.MultisigSig, error) {
	isValidKey := false
	for i := 0; i < len(partial.Subsigs); i++ {
		subsig := &partial.Subsigs[i]
		if subsig.Key == pk {
			isValidKey = true
			break
		}
	}

	if !isValidKey {
		return partial, errMsigWrongKey
	}

	sig, err := lw.signTransactionHelper(tx)
	if err != nil {
		return partial, err
	}

	for i := 0; i < len(partial.Subsigs); i++ {
		subsig := &partial.Subsigs[i]
		if subsig.Key == pk {
			subsig.Sig = sig
		}
	}

	return partial, nil
}

// MultisigSignProgram implements the Wallet interface.
func (lw *LedgerWallet) MultisigSignProgram(data []byte, src crypto.Digest, pk crypto.PublicKey, partial crypto.MultisigSig, pw []byte) (crypto.MultisigSig, error) {
	isValidKey := false
	for i := 0; i < len(partial.Subsigs); i++ {
		subsig := &partial.Subsigs[i]
		if subsig.Key == pk {
			isValidKey = true
			break
		}
	}

	if !isValidKey {
		return partial, errMsigWrongKey
	}

	sig, err := lw.signProgramHelper(data)
	if err != nil {
		return partial, err
	}

	for i := 0; i < len(partial.Subsigs); i++ {
		subsig := &partial.Subsigs[i]
		if subsig.Key == pk {
			subsig.Sig = sig
		}
	}

	return partial, nil
}

func uint64le(i uint64) []byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], i)
	return buf[:]
}

func (lw *LedgerWallet) signTransactionHelper(tx transactions.Transaction) (sig crypto.Signature, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	sig, err = lw.sendTransactionMsgpack(tx)
	if err == nil {
		return
	}

	ledgerErr, ok := err.(LedgerUSBError)
	if ok && ledgerErr == 0x6d00 {
		// We tried to send a msgpack-encoded transaction to the device,
		// but it doesn't support the new-style opcode, so fall back
		// to old-style encoding.
		sig, err = lw.sendTransactionOldStyle(tx)
	}

	return
}

func (lw *LedgerWallet) sendTransactionMsgpack(tx transactions.Transaction) (sig crypto.Signature, err error) {
	var reply []byte

	tosend := protocol.Encode(tx)
	p1 := ledgerP1first
	p2 := ledgerP2more

	// As a precaution, make sure that chunk + 5-byte APDU header
	// fits in 8-bit length fields.
	const chunkSize = 250

	for p2 != ledgerP2last {
		var chunk []byte
		if len(tosend) > chunkSize {
			chunk = tosend[:chunkSize]
		} else {
			chunk = tosend
			p2 = ledgerP2last
		}

		var msg []byte
		msg = append(msg, ledgerClass, ledgerInsSignMsgpack, p1, p2, uint8(len(chunk)))
		msg = append(msg, chunk...)

		reply, err = lw.dev.Exchange(msg)
		if err != nil {
			return
		}

		tosend = tosend[len(chunk):]
		p1 = ledgerP1more
	}

	if len(reply) > len(sig) {
		// Error related to transaction decoding.
		errmsg := string(reply[len(sig)+1:])
		err = errors.New(errmsg)
		return
	}

	copy(sig[:], reply)
	return
}

func (lw *LedgerWallet) sendTransactionOldStyle(tx transactions.Transaction) (sig crypto.Signature, err error) {
	var msg []byte
	msg = append(msg, ledgerClass)

	switch tx.Type {
	case protocol.PaymentTx:
		msg = append(msg, ledgerInsSignPaymentV2)
	case protocol.KeyRegistrationTx:
		msg = append(msg, ledgerInsSignKeyregV2)
	default:
		err = fmt.Errorf("transaction type %s not supported", tx.Type)
		return
	}

	if len(tx.Note) != 0 {
		err = fmt.Errorf("transaction notes not supported")
		return
	}

	msg = append(msg, tx.Sender[:]...)
	msg = append(msg, uint64le(tx.Fee.Raw)...)
	msg = append(msg, uint64le(uint64(tx.FirstValid))...)
	msg = append(msg, uint64le(uint64(tx.LastValid))...)

	var genbuf [32]byte
	if len(tx.GenesisID) > len(genbuf) {
		err = fmt.Errorf("genesis ID %s too long (%d)", tx.GenesisID, len(tx.GenesisID))
		return
	}

	copy(genbuf[:], []byte(tx.GenesisID))
	msg = append(msg, genbuf[:]...)
	msg = append(msg, tx.GenesisHash[:]...)

	switch tx.Type {
	case protocol.PaymentTx:
		msg = append(msg, tx.Receiver[:]...)
		msg = append(msg, uint64le(tx.Amount.Raw)...)
		msg = append(msg, tx.CloseRemainderTo[:]...)
	case protocol.KeyRegistrationTx:
		msg = append(msg, tx.VotePK[:]...)
		msg = append(msg, tx.SelectionPK[:]...)
	}

	reply, err := lw.dev.Exchange(msg)
	if err != nil {
		return
	}

	copy(sig[:], reply)
	return
}

func (lw *LedgerWallet) signProgramHelper(data []byte) (sig crypto.Signature, err error) {
	// TODO: extend client side code for signing program
	err = errors.New("signing programs not yet implemented for ledger wallet")
	return
}
