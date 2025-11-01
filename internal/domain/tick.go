package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// Tick represents a sequencer tick in the domain model.
// This is a clean domain type, independent of protobuf or database schemas.
type Tick struct {
	TickNumber   uint64        `json:"tick_number"`
	Timestamp    time.Time     `json:"timestamp"`
	VDFProof     VDFProof      `json:"vdf_proof"`
	Transactions []Transaction `json:"transactions"`
	BatchHash    string        `json:"batch_hash"`
	PrevOutput   string        `json:"previous_output"`
	ReceivedAt   time.Time     `json:"received_at"` // Added by ingester
}

// VDFProof represents a Verifiable Delay Function proof.
type VDFProof struct {
	Input      string `json:"input"`       // hex-encoded
	Output     string `json:"output"`      // hex-encoded
	Proof      string `json:"proof"`       // hex-encoded
	Iterations uint64 `json:"iterations"`
}

// Transaction represents an ordered transaction within a tick.
type Transaction struct {
	TxID               string    `json:"tx_id"`
	TxHash             string    `json:"tx_hash"`
	Payload            []byte    `json:"payload"`
	Signature          []byte    `json:"signature"`
	PublicKey          []byte    `json:"public_key"`
	Nonce              uint64    `json:"nonce"`
	ClientTimestamp    time.Time `json:"client_timestamp"`
	SequenceNumber     uint64    `json:"sequence_number"`
	IngestionTimestamp time.Time `json:"ingestion_timestamp"`
}

// Validate checks if the Tick has valid required fields.
func (t *Tick) Validate() error {
	if t.TickNumber == 0 {
		return fmt.Errorf("tick_number cannot be zero")
	}

	if t.Timestamp.IsZero() {
		return fmt.Errorf("timestamp cannot be zero")
	}

	if t.VDFProof.Output == "" {
		return fmt.Errorf("vdf_proof.output is required")
	}

	if t.BatchHash == "" {
		return fmt.Errorf("batch_hash is required")
	}

	return nil
}

// HasTransactions returns true if the tick contains any transactions.
func (t *Tick) HasTransactions() bool {
	return len(t.Transactions) > 0
}

// TransactionCount returns the number of transactions in this tick.
func (t *Tick) TransactionCount() int {
	return len(t.Transactions)
}

// MarshalJSON implements custom JSON marshaling.
func (t *Tick) MarshalJSON() ([]byte, error) {
	type Alias Tick
	return json.Marshal(&struct {
		*Alias
		Timestamp  string `json:"timestamp"`
		ReceivedAt string `json:"received_at"`
	}{
		Alias:      (*Alias)(t),
		Timestamp:  t.Timestamp.Format(time.RFC3339Nano),
		ReceivedAt: t.ReceivedAt.Format(time.RFC3339Nano),
	})
}

// Validate checks if the VDFProof has valid required fields.
func (v *VDFProof) Validate() error {
	if v.Input == "" {
		return fmt.Errorf("vdf_proof.input is required")
	}

	if v.Output == "" {
		return fmt.Errorf("vdf_proof.output is required")
	}

	if v.Proof == "" {
		return fmt.Errorf("vdf_proof.proof is required")
	}

	if v.Iterations == 0 {
		return fmt.Errorf("vdf_proof.iterations must be greater than zero")
	}

	return nil
}

// Validate checks if the Transaction has valid required fields.
func (tx *Transaction) Validate() error {
	if tx.TxHash == "" {
		return fmt.Errorf("tx_hash is required")
	}

	if len(tx.Signature) == 0 {
		return fmt.Errorf("signature is required")
	}

	if len(tx.PublicKey) == 0 {
		return fmt.Errorf("public_key is required")
	}

	return nil
}
