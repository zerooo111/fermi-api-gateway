package parser

import (
	"fmt"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
)

// ProtobufParser converts protobuf ticks to domain model ticks.
// It is stateless and safe for concurrent use.
type ProtobufParser struct{}

// NewProtobufParser creates a new protobuf parser.
func NewProtobufParser() *ProtobufParser {
	return &ProtobufParser{}
}

// Parse converts a protobuf tick to a domain tick.
func (p *ProtobufParser) Parse(pbTick *pb.Tick) (*domain.Tick, error) {
	if pbTick == nil {
		return nil, fmt.Errorf("tick cannot be nil")
	}

	// Parse VDF proof
	vdfProof, err := p.parseVDFProof(pbTick.VdfProof)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vdf_proof: %w", err)
	}

	// Parse transactions
	transactions, err := p.parseTransactions(pbTick.Transactions)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transactions: %w", err)
	}

	// Convert timestamp from microseconds to time.Time
	timestamp := time.UnixMicro(int64(pbTick.Timestamp))

	// Create domain tick
	tick := &domain.Tick{
		TickNumber:   pbTick.TickNumber,
		Timestamp:    timestamp,
		VDFProof:     vdfProof,
		Transactions: transactions,
		BatchHash:    pbTick.TransactionBatchHash,
		PrevOutput:   pbTick.PreviousOutput,
		ReceivedAt:   time.Now(), // Ingestion timestamp
	}

	// Validate the domain tick
	if err := tick.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tick: %w", err)
	}

	return tick, nil
}

// parseVDFProof converts protobuf VdfProof to domain VDFProof.
func (p *ProtobufParser) parseVDFProof(pbProof *pb.VdfProof) (domain.VDFProof, error) {
	if pbProof == nil {
		return domain.VDFProof{}, fmt.Errorf("vdf_proof cannot be nil")
	}

	proof := domain.VDFProof{
		Input:      pbProof.Input,
		Output:     pbProof.Output,
		Proof:      pbProof.Proof,
		Iterations: pbProof.Iterations,
	}

	// Validate the VDF proof
	if err := proof.Validate(); err != nil {
		return domain.VDFProof{}, err
	}

	return proof, nil
}

// parseTransactions converts protobuf OrderedTransactions to domain Transactions.
func (p *ProtobufParser) parseTransactions(pbTxs []*pb.OrderedTransaction) ([]domain.Transaction, error) {
	// Empty transaction list is valid
	if len(pbTxs) == 0 {
		return []domain.Transaction{}, nil
	}

	transactions := make([]domain.Transaction, 0, len(pbTxs))

	for i, pbOrderedTx := range pbTxs {
		if pbOrderedTx == nil {
			return nil, fmt.Errorf("transaction at index %d is nil", i)
		}

		if pbOrderedTx.Transaction == nil {
			return nil, fmt.Errorf("transaction.transaction at index %d is nil", i)
		}

		pbTx := pbOrderedTx.Transaction

		// Convert client timestamp from microseconds to time.Time
		clientTimestamp := time.UnixMicro(int64(pbTx.Timestamp))

		// Convert ingestion timestamp from microseconds to time.Time
		ingestionTimestamp := time.UnixMicro(int64(pbOrderedTx.IngestionTimestamp))

		tx := domain.Transaction{
			TxID:               pbTx.TxId,
			TxHash:             pbOrderedTx.TxHash,
			Payload:            pbTx.Payload,
			Signature:          pbTx.Signature,
			PublicKey:          pbTx.PublicKey,
			Nonce:              pbTx.Nonce,
			ClientTimestamp:    clientTimestamp,
			SequenceNumber:     pbOrderedTx.SequenceNumber,
			IngestionTimestamp: ingestionTimestamp,
		}

		// Validate the transaction
		if err := tx.Validate(); err != nil {
			return nil, fmt.Errorf("invalid transaction at index %d: %w", i, err)
		}

		transactions = append(transactions, tx)
	}

	return transactions, nil
}
