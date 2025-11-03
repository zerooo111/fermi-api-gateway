package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	// Parse command-line flags
	serverAddr := flag.String("server", "100.24.216.168:9090", "gRPC server address")
	tickLimit := flag.Uint("tick-limit", 10, "Number of recent ticks to request")
	rawOutput := flag.Bool("raw", false, "Show raw JSON output")
	flag.Parse()

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  GetChainState Test Tool\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("Server:     %s\n", *serverAddr)
	fmt.Printf("Tick Limit: %d\n", *tickLimit)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to gRPC server
	fmt.Printf("[%s] Connecting to gRPC server...\n", time.Now().Format("15:04:05.000"))
	conn, err := grpc.NewClient(
		*serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB
		),
	)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create gRPC connection: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("[%s] ✓ Connection established\n", time.Now().Format("15:04:05.000"))

	// Create client
	client := pb.NewSequencerServiceClient(conn)

	// Call GetChainState
	fmt.Printf("[%s] Calling GetChainState...\n\n", time.Now().Format("15:04:05.000"))

	resp, err := client.GetChainState(ctx, &pb.GetChainStateRequest{
		TickLimit: uint32(*tickLimit),
	})
	if err != nil {
		fmt.Printf("[ERROR] GetChainState failed: %v\n", err)
		if st, ok := status.FromError(err); ok {
			fmt.Printf("  Status Code: %v\n", st.Code())
			fmt.Printf("  Status Message: %s\n", st.Message())
		}
		os.Exit(1)
	}

	// Print raw JSON if requested
	if *rawOutput {
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		fmt.Printf("  Raw JSON Response\n")
		fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

		// Convert to JSON using protojson for proper formatting
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		jsonBytes, err := marshaler.Marshal(resp)
		if err != nil {
			fmt.Printf("[ERROR] Failed to marshal to JSON: %v\n", err)
			os.Exit(1)
		}

		// Pretty print
		var prettyJSON map[string]interface{}
		json.Unmarshal(jsonBytes, &prettyJSON)
		prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
		fmt.Println(string(prettyBytes))
		fmt.Printf("\n═══════════════════════════════════════════════════════════════\n")
		return
	}

	// Print response
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  Chain State Response\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

	fmt.Printf("Chain Height:        %d\n", resp.ChainHeight)
	fmt.Printf("Total Transactions:  %d\n", resp.TotalTransactions)
	fmt.Printf("Recent Ticks Count:  %d\n", len(resp.RecentTicks))
	fmt.Printf("TX Sample Count:     %d\n\n", len(resp.TxToTickSample))

	if len(resp.RecentTicks) > 0 {
		fmt.Printf("───────────────────────────────────────────────────────────────\n")
		fmt.Printf("Recent Ticks:\n")
		fmt.Printf("───────────────────────────────────────────────────────────────\n\n")

		for i, tick := range resp.RecentTicks {
			fmt.Printf("[%d] Tick #%d\n", i+1, tick.TickNumber)
			fmt.Printf("    Timestamp:    %d\n", tick.Timestamp)
			fmt.Printf("    Transactions: %d\n", len(tick.Transactions))
			fmt.Printf("    Batch Hash:   %s\n", truncateString(tick.TransactionBatchHash, 32))
			fmt.Printf("    Prev Output:  %s\n", truncateString(tick.PreviousOutput, 32))

			if tick.VdfProof != nil {
				fmt.Printf("    VDF Proof:\n")
				fmt.Printf("      Input:      %s\n", truncateString(tick.VdfProof.Input, 32))
				fmt.Printf("      Output:     %s\n", truncateString(tick.VdfProof.Output, 32))
				fmt.Printf("      Iterations: %d\n", tick.VdfProof.Iterations)
			}

			if len(tick.Transactions) > 0 {
				fmt.Printf("    Transactions:\n")
				for j, tx := range tick.Transactions {
					fmt.Printf("      [%d] Hash: %s | Seq: %d\n",
						j+1,
						truncateString(tx.TxHash, 16),
						tx.SequenceNumber,
					)
				}
			}

			fmt.Println()
		}
	}

	if len(resp.TxToTickSample) > 0 {
		fmt.Printf("───────────────────────────────────────────────────────────────\n")
		fmt.Printf("Transaction → Tick Sample (showing %d entries):\n", len(resp.TxToTickSample))
		fmt.Printf("───────────────────────────────────────────────────────────────\n\n")

		count := 0
		for txHash, tickNum := range resp.TxToTickSample {
			fmt.Printf("  %s → Tick #%d\n", truncateString(txHash, 32), tickNum)
			count++
			if count >= 10 {
				if len(resp.TxToTickSample) > 10 {
					fmt.Printf("  ... and %d more\n", len(resp.TxToTickSample)-10)
				}
				break
			}
		}
		fmt.Println()
	}

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
}

// Helper function
func truncateString(s string, maxLen int) string {
	if s == "" {
		return "(empty)"
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
