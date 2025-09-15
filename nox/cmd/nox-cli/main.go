package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	pb "nox/proto"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	serverAddr string
)

var rootCmd = &cobra.Command{
	Use:   "nox-cli",
	Short: "A gRPC client for the Nox IDS engine.",
	Long:  `Nox CLI is a tool to interact with the nox gRPC API for threat hunting and data exploration`,
}

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for process execution events.",
	Run: func(cmd *cobra.Command, args []string) {
		startTimeStr, _ := cmd.Flags().GetString("start-time")
		endTimeStr, _ := cmd.Flags().GetString("end-time")
		filters, _ := cmd.Flags().GetStringToString("filter")

		var startTime, endTime time.Time
		var err error

		if startTimeStr != "" {
			startTime, err = time.Parse(time.RFC3339, startTimeStr)
			if err != nil {
				log.Fatal("Invalid start-time format. Use RFC3339(e.g., '2023-01-01T15:04:05Z'): %v", err)
			}
		}

		if endTimeStr != "" {
			endTime, err = time.Parse(time.RFC3339, endTimeStr)
			if err != nil {
				log.Fatalf("Invalid end-time format. Use RFC3339 (e.g., '2023-01-01T15:04:05Z'): %v", err)
			}
		}

		c, conn := connect()
		defer conn.Close()

		req := &pb.SearchRequest{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
			Filters:   filters,
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		res, err := c.SearchEvents(ctx, req)
		if err != nil {
			log.Fatal("Could not perform search: %v", err)
		}

		log.Printf("Found %d events: ", len(res.ProcessEvents))
		for _, event := range res.ProcessEvents {
			fmt.Printf("  - Time: %s, PID: %s, PPID: %s, UID: %s, Cmd: %s\n",
				event.Timestamp.AsTime().Format(time.RFC822), event.Pid, event.Ppid, event.Uid, event.Command)
		}
	},
}

var ancestryCmd = &cobra.Command{
	Use:   "ancestry [pid]",
	Short: "Get the process ancestry for a given PID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pid := args[0]
		c, conn := connect()
		defer conn.Close()

		req := &pb.PIDRequest{Pid: pid}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		res, err := c.GetProcessAncestry(ctx, req)
		if err != nil {
			log.Fatal("Could not get process ancestry: %v", err)
		}

		log.Printf("Process Ancestry for PID %s (newest first): ", pid)
		for _, event := range res.Events {
			fmt.Printf("  - PID: %-7s PPID: %-7s Cmd: %s\n", event.Pid, event.Ppid, event.Command)
		}
	},
}

var topCmd = &cobra.Command{
	Use:   "top [field]",
	Short: "Get the top N most frequent values for a field",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		field := args[0]
		n, _ := cmd.Flags().GetInt32("n")

		c, conn := connect()
		defer conn.Close()

		req := &pb.TopNRequest{Field: field, N: n}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		res, err := c.GetTopEvents(ctx, req)
		if err != nil {
			log.Fatalf("Could not get top events: %v", err)
		}

		log.Printf("Top %d values for field '%s': ", n, field)
		for _, result := range res.Results {
			fmt.Printf("  - %-30s Count: %d\n", result.Item, result.Count)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverAddr, "addr", "localhost:50051", "The server address in the format of host:port")
	searchCmd.Flags().String("start-time", "", "Start time in RFC3339 format")
	searchCmd.Flags().String("end-time", "", "End time in RFC3339 format")
	searchCmd.Flags().StringToString("filter", nil, "Metadata filters (e.g., --filter process_name=bash)")
	topCmd.Flags().Int32P("n", "n", 10, "The number of top results to return")
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(ancestryCmd)
	rootCmd.AddCommand(topCmd)
}

func connect() (pb.NoxServiceClient, *grpc.ClientConn) {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect: %v", err)
	}

	return pb.NewNoxServiceClient(conn), conn
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
