package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTick_Validate(t *testing.T) {
	validTick := func() *Tick {
		return &Tick{
			TickNumber: 12345,
			Timestamp:  time.Now(),
			VDFProof: VDFProof{
				Input:      "abc123",
				Output:     "def456",
				Proof:      "proof789",
				Iterations: 1000,
			},
			BatchHash:  "hash123",
			PrevOutput: "prev456",
			ReceivedAt: time.Now(),
		}
	}

	tests := []struct {
		name    string
		tick    *Tick
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid tick",
			tick:    validTick(),
			wantErr: false,
		},
		{
			name: "zero tick_number",
			tick: func() *Tick {
				tick := validTick()
				tick.TickNumber = 0
				return tick
			}(),
			wantErr: true,
			errMsg:  "tick_number cannot be zero",
		},
		{
			name: "zero timestamp",
			tick: func() *Tick {
				tick := validTick()
				tick.Timestamp = time.Time{}
				return tick
			}(),
			wantErr: true,
			errMsg:  "timestamp cannot be zero",
		},
		{
			name: "empty vdf_proof.output",
			tick: func() *Tick {
				tick := validTick()
				tick.VDFProof.Output = ""
				return tick
			}(),
			wantErr: true,
			errMsg:  "vdf_proof.output is required",
		},
		{
			name: "empty batch_hash",
			tick: func() *Tick {
				tick := validTick()
				tick.BatchHash = ""
				return tick
			}(),
			wantErr: true,
			errMsg:  "batch_hash is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tick.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Tick.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Tick.Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestTick_HasTransactions(t *testing.T) {
	tests := []struct {
		name string
		tick *Tick
		want bool
	}{
		{
			name: "tick with transactions",
			tick: &Tick{
				Transactions: []Transaction{
					{TxHash: "tx1"},
					{TxHash: "tx2"},
				},
			},
			want: true,
		},
		{
			name: "tick without transactions",
			tick: &Tick{
				Transactions: []Transaction{},
			},
			want: false,
		},
		{
			name: "tick with nil transactions",
			tick: &Tick{
				Transactions: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tick.HasTransactions(); got != tt.want {
				t.Errorf("Tick.HasTransactions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTick_TransactionCount(t *testing.T) {
	tests := []struct {
		name string
		tick *Tick
		want int
	}{
		{
			name: "tick with 3 transactions",
			tick: &Tick{
				Transactions: []Transaction{{}, {}, {}},
			},
			want: 3,
		},
		{
			name: "tick with 0 transactions",
			tick: &Tick{
				Transactions: []Transaction{},
			},
			want: 0,
		},
		{
			name: "tick with nil transactions",
			tick: &Tick{
				Transactions: nil,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tick.TransactionCount(); got != tt.want {
				t.Errorf("Tick.TransactionCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTick_MarshalJSON(t *testing.T) {
	tick := &Tick{
		TickNumber: 12345,
		Timestamp:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		VDFProof: VDFProof{
			Input:      "input123",
			Output:     "output456",
			Proof:      "proof789",
			Iterations: 1000,
		},
		BatchHash:    "hash123",
		PrevOutput:   "prev456",
		ReceivedAt:   time.Date(2025, 1, 1, 12, 0, 1, 0, time.UTC),
		Transactions: []Transaction{},
	}

	data, err := json.Marshal(tick)
	if err != nil {
		t.Fatalf("failed to marshal tick: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal tick JSON: %v", err)
	}

	// Check timestamp is in RFC3339Nano format
	if timestamp, ok := result["timestamp"].(string); !ok {
		t.Errorf("timestamp is not a string")
	} else if timestamp != "2025-01-01T12:00:00Z" {
		t.Errorf("timestamp = %v, want RFC3339Nano format", timestamp)
	}

	// Check tick_number
	if tickNum, ok := result["tick_number"].(float64); !ok {
		t.Errorf("tick_number is not a number")
	} else if uint64(tickNum) != 12345 {
		t.Errorf("tick_number = %v, want 12345", tickNum)
	}
}

func TestVDFProof_Validate(t *testing.T) {
	validProof := func() *VDFProof {
		return &VDFProof{
			Input:      "input123",
			Output:     "output456",
			Proof:      "proof789",
			Iterations: 1000,
		}
	}

	tests := []struct {
		name    string
		proof   *VDFProof
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid proof",
			proof:   validProof(),
			wantErr: false,
		},
		{
			name: "empty input",
			proof: func() *VDFProof {
				p := validProof()
				p.Input = ""
				return p
			}(),
			wantErr: true,
			errMsg:  "vdf_proof.input is required",
		},
		{
			name: "empty output",
			proof: func() *VDFProof {
				p := validProof()
				p.Output = ""
				return p
			}(),
			wantErr: true,
			errMsg:  "vdf_proof.output is required",
		},
		{
			name: "empty proof",
			proof: func() *VDFProof {
				p := validProof()
				p.Proof = ""
				return p
			}(),
			wantErr: true,
			errMsg:  "vdf_proof.proof is required",
		},
		{
			name: "zero iterations",
			proof: func() *VDFProof {
				p := validProof()
				p.Iterations = 0
				return p
			}(),
			wantErr: true,
			errMsg:  "vdf_proof.iterations must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.proof.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("VDFProof.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("VDFProof.Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestTransaction_Validate(t *testing.T) {
	validTx := func() *Transaction {
		return &Transaction{
			TxHash:    "hash123",
			Signature: []byte("signature"),
			PublicKey: []byte("pubkey"),
		}
	}

	tests := []struct {
		name    string
		tx      *Transaction
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid transaction",
			tx:      validTx(),
			wantErr: false,
		},
		{
			name: "empty tx_hash",
			tx: func() *Transaction {
				tx := validTx()
				tx.TxHash = ""
				return tx
			}(),
			wantErr: true,
			errMsg:  "tx_hash is required",
		},
		{
			name: "empty signature",
			tx: func() *Transaction {
				tx := validTx()
				tx.Signature = []byte{}
				return tx
			}(),
			wantErr: true,
			errMsg:  "signature is required",
		},
		{
			name: "nil signature",
			tx: func() *Transaction {
				tx := validTx()
				tx.Signature = nil
				return tx
			}(),
			wantErr: true,
			errMsg:  "signature is required",
		},
		{
			name: "empty public_key",
			tx: func() *Transaction {
				tx := validTx()
				tx.PublicKey = []byte{}
				return tx
			}(),
			wantErr: true,
			errMsg:  "public_key is required",
		},
		{
			name: "nil public_key",
			tx: func() *Transaction {
				tx := validTx()
				tx.PublicKey = nil
				return tx
			}(),
			wantErr: true,
			errMsg:  "public_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tx.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Transaction.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Transaction.Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}
