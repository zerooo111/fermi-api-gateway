package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func main() {
	// Parse command-line flags
	serverAddr := flag.String("server", "100.24.216.168:9090", "gRPC server address")
	startTick := flag.Uint64("start-tick", 0, "Starting tick number (0 = latest)")
	verbose := flag.Bool("verbose", false, "Verbose output (show each tick)")
	flag.Parse()

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  gRPC Stream Test Tool\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("Server:     %s\n", *serverAddr)
	fmt.Printf("Start Tick: %d\n", *startTick)
	fmt.Printf("Verbose:    %t\n", *verbose)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\n[SIGNAL] Received interrupt, shutting down...")
		cancel()
	}()

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

	// Start streaming
	fmt.Printf("[%s] Starting tick stream (start_tick=%d)...\n", time.Now().Format("15:04:05.000"), *startTick)

	// Add metadata if needed
	ctx = metadata.AppendToOutgoingContext(ctx)

	stream, err := client.StreamTicks(ctx, &pb.StreamTicksRequest{
		StartTick: *startTick,
	})
	if err != nil {
		fmt.Printf("[ERROR] Failed to start stream: %v\n", err)
		if st, ok := status.FromError(err); ok {
			fmt.Printf("  Status Code: %v\n", st.Code())
			fmt.Printf("  Status Message: %s\n", st.Message())
		}
		os.Exit(1)
	}

	fmt.Printf("[%s] ✓ Stream started successfully\n", time.Now().Format("15:04:05.000"))
	fmt.Printf("[%s] Waiting for ticks...\n\n", time.Now().Format("15:04:05.000"))

	// Read from stream
	tickCount := uint64(0)
	startTime := time.Now()
	lastPrintTime := startTime

	for {
		tick, err := stream.Recv()
		if err != nil {
			elapsed := time.Since(startTime)

			if err == io.EOF {
				fmt.Printf("\n[%s] Stream closed by server (EOF)\n", time.Now().Format("15:04:05.000"))
				fmt.Printf("  Duration: %v\n", elapsed)
				fmt.Printf("  Ticks received: %d\n", tickCount)
				if tickCount > 0 {
					fmt.Printf("  Rate: %.2f ticks/sec\n", float64(tickCount)/elapsed.Seconds())
				}
				return
			}

			fmt.Printf("\n[ERROR] Stream error: %v\n", err)
			fmt.Printf("  Duration before error: %v\n", elapsed)
			fmt.Printf("  Ticks received: %d\n", tickCount)

			// Check gRPC status
			if st, ok := status.FromError(err); ok {
				fmt.Printf("  gRPC Status Code: %v\n", st.Code())
				fmt.Printf("  gRPC Status Message: %s\n", st.Message())
			}

			os.Exit(1)
		}

		// Increment counter
		tickCount++

		// Print tick info
		if *verbose {
			fmt.Printf("[TICK] #%d | Tick: %d | Txns: %d | BatchHash: %s | Time: %d\n",
				tickCount,
				tick.TickNumber,
				len(tick.Transactions),
				truncateString(tick.TransactionBatchHash, 16),
				tick.Timestamp,
			)
		} else {
			// Print summary every second
			now := time.Now()
			if now.Sub(lastPrintTime) >= time.Second {
				elapsed := now.Sub(startTime)
				rate := float64(tickCount) / elapsed.Seconds()
				fmt.Printf("[%s] Ticks: %d | Rate: %.2f ticks/sec | Last Tick: %d\n",
					now.Format("15:04:05"),
					tickCount,
					rate,
					tick.TickNumber,
				)
				lastPrintTime = now
			}
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			fmt.Printf("\n[%s] Stream stopped by user\n", time.Now().Format("15:04:05.000"))
			fmt.Printf("  Duration: %v\n", elapsed)
			fmt.Printf("  Ticks received: %d\n", tickCount)
			if tickCount > 0 {
				fmt.Printf("  Rate: %.2f ticks/sec\n", float64(tickCount)/elapsed.Seconds())
			}
			return
		default:
		}
	}
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
