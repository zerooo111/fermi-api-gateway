package parser

import (
	"testing"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
)

func TestProtobufParser_Parse(t *testing.T) {
	validPbTick := func() *pb.Tick {
		return &pb.Tick{
			TickNumber: 12345,
			VdfProof: &pb.VdfProof{
				Input:      "input123",
				Output:     "output456",
				Proof:      "proof789",
				Iterations: 1000,
			},
			Transactions:         []*pb.OrderedTransaction{},
			TransactionBatchHash: "batch_hash_123",
			Timestamp:            uint64(time.Now().UnixMicro()),
			PreviousOutput:       "prev_output_456",
		}
	}

	tests := []struct {
		name    string
		tick    *pb.Tick
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid tick without transactions",
			tick:    validPbTick(),
			wantErr: false,
		},
		{
			name: "valid tick with transactions",
			tick: func() *pb.Tick {
				tick := validPbTick()
				tick.Transactions = []*pb.OrderedTransaction{
					{
						Transaction: &pb.Transaction{
							TxId:      "tx1",
							Payload:   []byte("payload1"),
							Signature: []byte("sig1"),
							PublicKey: []byte("pubkey1"),
							Nonce:     1,
							Timestamp: uint64(time.Now().UnixMicro()),
						},
						SequenceNumber:     1,
						TxHash:             "hash1",
						IngestionTimestamp: uint64(time.Now().UnixMicro()),
					},
				}
				return tick
			}(),
			wantErr: false,
		},
		{
			name:    "nil tick",
			tick:    nil,
			wantErr: true,
			errMsg:  "tick cannot be nil",
		},
		{
			name: "nil vdf_proof",
			tick: func() *pb.Tick {
				tick := validPbTick()
				tick.VdfProof = nil
				return tick
			}(),
			wantErr: true,
			errMsg:  "failed to parse vdf_proof",
		},
		{
			name: "invalid vdf_proof (empty output)",
			tick: func() *pb.Tick {
				tick := validPbTick()
				tick.VdfProof.Output = ""
				return tick
			}(),
			wantErr: true,
			errMsg:  "failed to parse vdf_proof",
		},
		{
			name: "nil transaction in array",
			tick: func() *pb.Tick {
				tick := validPbTick()
				tick.Transactions = []*pb.OrderedTransaction{nil}
				return tick
			}(),
			wantErr: true,
			errMsg:  "transaction at index 0 is nil",
		},
		{
			name: "ordered transaction with nil inner transaction",
			tick: func() *pb.Tick {
				tick := validPbTick()
				tick.Transactions = []*pb.OrderedTransaction{
					{
						Transaction:        nil,
						SequenceNumber:     1,
						TxHash:             "hash1",
						IngestionTimestamp: uint64(time.Now().UnixMicro()),
					},
				}
				return tick
			}(),
			wantErr: true,
			errMsg:  "transaction.transaction at index 0 is nil",
		},
		{
			name: "invalid transaction (empty tx_hash)",
			tick: func() *pb.Tick {
				tick := validPbTick()
				tick.Transactions = []*pb.OrderedTransaction{
					{
						Transaction: &pb.Transaction{
							TxId:      "tx1",
							Payload:   []byte("payload1"),
							Signature: []byte("sig1"),
							PublicKey: []byte("pubkey1"),
							Nonce:     1,
							Timestamp: uint64(time.Now().UnixMicro()),
						},
						SequenceNumber:     1,
						TxHash:             "", // Empty hash
						IngestionTimestamp: uint64(time.Now().UnixMicro()),
					},
				}
				return tick
			}(),
			wantErr: true,
			errMsg:  "invalid transaction at index 0",
		},
		{
			name: "zero tick_number (invalid domain tick)",
			tick: func() *pb.Tick {
				tick := validPbTick()
				tick.TickNumber = 0
				return tick
			}(),
			wantErr: true,
			errMsg:  "invalid tick",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewProtobufParser()
			tick, err := parser.Parse(tt.tick)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error containing %q, got nil", tt.errMsg)
				}
				return
			}

			// Verify successful parse
			if tick == nil {
				t.Errorf("Parse() returned nil tick for valid input")
				return
			}

			// Verify tick number
			if tick.TickNumber != tt.tick.TickNumber {
				t.Errorf("Parse() tick_number = %v, want %v", tick.TickNumber, tt.tick.TickNumber)
			}

			// Verify batch hash
			if tick.BatchHash != tt.tick.TransactionBatchHash {
				t.Errorf("Parse() batch_hash = %v, want %v", tick.BatchHash, tt.tick.TransactionBatchHash)
			}

			// Verify previous output
			if tick.PrevOutput != tt.tick.PreviousOutput {
				t.Errorf("Parse() prev_output = %v, want %v", tick.PrevOutput, tt.tick.PreviousOutput)
			}

			// Verify VDF proof
			if tick.VDFProof.Input != tt.tick.VdfProof.Input {
				t.Errorf("Parse() vdf_proof.input = %v, want %v", tick.VDFProof.Input, tt.tick.VdfProof.Input)
			}

			// Verify transaction count
			if len(tick.Transactions) != len(tt.tick.Transactions) {
				t.Errorf("Parse() transaction count = %v, want %v", len(tick.Transactions), len(tt.tick.Transactions))
			}

			// Verify ReceivedAt is set
			if tick.ReceivedAt.IsZero() {
				t.Errorf("Parse() ReceivedAt is zero, should be set to current time")
			}

			// Verify timestamp conversion
			expectedTime := time.UnixMicro(int64(tt.tick.Timestamp))
			if !tick.Timestamp.Equal(expectedTime) {
				t.Errorf("Parse() timestamp = %v, want %v", tick.Timestamp, expectedTime)
			}
		})
	}
}

func TestProtobufParser_ParseTransactions(t *testing.T) {
	parser := NewProtobufParser()

	t.Run("empty transaction list", func(t *testing.T) {
		txs, err := parser.parseTransactions([]*pb.OrderedTransaction{})
		if err != nil {
			t.Errorf("parseTransactions() error = %v, want nil", err)
		}
		if len(txs) != 0 {
			t.Errorf("parseTransactions() length = %v, want 0", len(txs))
		}
	})

	t.Run("multiple valid transactions", func(t *testing.T) {
		pbTxs := []*pb.OrderedTransaction{
			{
				Transaction: &pb.Transaction{
					TxId:      "tx1",
					Payload:   []byte("payload1"),
					Signature: []byte("sig1"),
					PublicKey: []byte("pubkey1"),
					Nonce:     1,
					Timestamp: uint64(time.Now().UnixMicro()),
				},
				SequenceNumber:     1,
				TxHash:             "hash1",
				IngestionTimestamp: uint64(time.Now().UnixMicro()),
			},
			{
				Transaction: &pb.Transaction{
					TxId:      "tx2",
					Payload:   []byte("payload2"),
					Signature: []byte("sig2"),
					PublicKey: []byte("pubkey2"),
					Nonce:     2,
					Timestamp: uint64(time.Now().UnixMicro()),
				},
				SequenceNumber:     2,
				TxHash:             "hash2",
				IngestionTimestamp: uint64(time.Now().UnixMicro()),
			},
		}

		txs, err := parser.parseTransactions(pbTxs)
		if err != nil {
			t.Fatalf("parseTransactions() error = %v, want nil", err)
		}

		if len(txs) != 2 {
			t.Fatalf("parseTransactions() length = %v, want 2", len(txs))
		}

		// Verify first transaction
		if txs[0].TxID != "tx1" {
			t.Errorf("txs[0].TxID = %v, want tx1", txs[0].TxID)
		}
		if txs[0].TxHash != "hash1" {
			t.Errorf("txs[0].TxHash = %v, want hash1", txs[0].TxHash)
		}

		// Verify second transaction
		if txs[1].TxID != "tx2" {
			t.Errorf("txs[1].TxID = %v, want tx2", txs[1].TxID)
		}
		if txs[1].SequenceNumber != 2 {
			t.Errorf("txs[1].SequenceNumber = %v, want 2", txs[1].SequenceNumber)
		}
	})
}

func TestProtobufParser_ParseVDFProof(t *testing.T) {
	parser := NewProtobufParser()

	tests := []struct {
		name    string
		proof   *pb.VdfProof
		wantErr bool
	}{
		{
			name: "valid proof",
			proof: &pb.VdfProof{
				Input:      "input123",
				Output:     "output456",
				Proof:      "proof789",
				Iterations: 1000,
			},
			wantErr: false,
		},
		{
			name:    "nil proof",
			proof:   nil,
			wantErr: true,
		},
		{
			name: "empty input",
			proof: &pb.VdfProof{
				Input:      "",
				Output:     "output456",
				Proof:      "proof789",
				Iterations: 1000,
			},
			wantErr: true,
		},
		{
			name: "zero iterations",
			proof: &pb.VdfProof{
				Input:      "input123",
				Output:     "output456",
				Proof:      "proof789",
				Iterations: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proof, err := parser.parseVDFProof(tt.proof)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseVDFProof() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if proof.Input != tt.proof.Input {
					t.Errorf("proof.Input = %v, want %v", proof.Input, tt.proof.Input)
				}
				if proof.Iterations != tt.proof.Iterations {
					t.Errorf("proof.Iterations = %v, want %v", proof.Iterations, tt.proof.Iterations)
				}
			}
		})
	}
}
