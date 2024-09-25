/*
Extended DNS Check (chk) is a command-line tool that provides enhanced DNS lookup functionality.
It checks PTR records for IP addresses and fetches additional IP information using the ipinfo.io API.
For domains or subdomains, it displays A and AAAA records and retrieves IP information for each resolved address.

Features:
  - Lookup A and AAAA records for domains and subdomains
  - Retrieve PTR records for IP addresses
  - Fetch detailed IP information (city, region, country, etc.) using ipinfo.io API
  - Support for IPv4 and IPv6 addresses
  - Colorized output for better readability
  - Parallel processing using goroutines
  - Configurable timeout handling for HTTP requests
  - Graceful shutdown on user interrupt
  - Progress information during execution

GitHub Repository: https://github.com/kofany/chk

Author: Jerzy Dąbrowski

License: MIT License (https://kofany.mit-license.org)
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
)

var CLI struct {
	IPv4    bool          `help:"Show only IPv4 (A) records" short:"4"`
	IPv6    bool          `help:"Show only IPv6 (AAAA) records" short:"6"`
	Timeout time.Duration `help:"Timeout for HTTP requests" default:"5s"`
	Target  string        `arg name:"domain/ip" help:"Domain, subdomain or IP to check"`
}

type IPInfo struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
}

type Result struct {
	IP     string
	PTR    []string
	IPInfo *IPInfo
	IsIPv6 bool
	Error  error
}

var (
	cyan    = color.New(color.FgCyan).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
)

var httpClient *http.Client

func getIPInfo(ctx context.Context, ip string) (*IPInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://ipinfo.io/"+ip+"/json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ipInfo IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&ipInfo); err != nil {
		return nil, err
	}
	return &ipInfo, nil
}

func lookupIP(ctx context.Context, ip string, isIPv6 bool, resultChan chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	result := Result{IP: ip, IsIPv6: isIPv6}

	var wgInternal sync.WaitGroup
	wgInternal.Add(2)

	go func() {
		defer wgInternal.Done()
		names, err := net.LookupAddr(ip)
		if err != nil {
			result.Error = fmt.Errorf("error looking up PTR records: %v", err)
		} else {
			result.PTR = names
		}
	}()

	go func() {
		defer wgInternal.Done()
		ipInfo, err := getIPInfo(ctx, ip)
		if err != nil {
			if result.Error != nil {
				result.Error = fmt.Errorf("%v; error fetching IP info: %v", result.Error, err)
			} else {
				result.Error = fmt.Errorf("error fetching IP info: %v", err)
			}
		} else {
			result.IPInfo = ipInfo
		}
	}()

	wgInternal.Wait()
	resultChan <- result
}

func printResult(result Result) {
	recordType := "A"
	if result.IsIPv6 {
		recordType = "AAAA"
	}
	fmt.Printf("%s: %s\n", cyan(fmt.Sprintf("%s Record", recordType)), yellow(result.IP))

	if len(result.PTR) > 0 {
		fmt.Printf("  %s: %s\n", cyan("PTR Records"), green(strings.Join(result.PTR, ", ")))
	}

	if result.IPInfo != nil {
		fmt.Printf("  %s: %s\n", cyan("City"), green(result.IPInfo.City))
		fmt.Printf("  %s: %s\n", cyan("Region"), green(result.IPInfo.Region))
		fmt.Printf("  %s: %s\n", cyan("Country"), green(result.IPInfo.Country))
		fmt.Printf("  %s: %s\n", cyan("Location"), green(result.IPInfo.Loc))
		fmt.Printf("  %s: %s\n", cyan("Organization"), green(result.IPInfo.Org))
	}

	if result.Error != nil {
		fmt.Printf("  %s: %s\n", red("Error"), red(result.Error.Error()))
	}

	fmt.Println()
}

func validateInput(input string) error {
	if net.ParseIP(input) != nil {
		return nil
	}
	if _, err := net.LookupHost(input); err != nil {
		return fmt.Errorf("invalid domain or IP address: %v", err)
	}
	return nil
}

func main() {
	ctx := kong.Parse(&CLI)

	if err := validateInput(CLI.Target); err != nil {
		fmt.Printf("%s: %v\n", red("Error"), red(err))
		ctx.Exit(1)
	}

	httpClient = &http.Client{Timeout: CLI.Timeout}

	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupt received, shutting down...")
		cancel()
	}()

	ip := net.ParseIP(CLI.Target)

	var wg sync.WaitGroup
	resultChan := make(chan Result, 10) // Buffered channel
	var results []Result

	var ips []net.IP
	if ip != nil {
		ips = append(ips, ip)
	} else {
		var err error
		ips, err = net.LookupIP(CLI.Target)
		if err != nil {
			fmt.Printf("%s: %v\n", red("Error looking up IP for domain"), red(err))
			ctx.Exit(1)
		}
	}

	totalIPs := 0
	for _, ip := range ips {
		isIPv6 := ip.To4() == nil
		if (CLI.IPv4 && !isIPv6) || (CLI.IPv6 && isIPv6) || (!CLI.IPv4 && !CLI.IPv6) {
			totalIPs++
			wg.Add(1)
			go lookupIP(mainCtx, ip.String(), isIPv6, resultChan, &wg)
		}
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	done := make(chan bool)
	go func() {
		for result := range resultChan {
			results = append(results, result)
		}
		close(done)
	}()

	fmt.Print(yellow("Checking records... Please wait"))
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	processed := 0
	for {
		select {
		case <-mainCtx.Done():
			fmt.Println("\nOperation cancelled")
			return
		case <-done:
			fmt.Print("\r" + strings.Repeat(" ", 60) + "\r") // Clear the progress message
			for _, result := range results {
				printResult(result)
			}
			return
		case <-ticker.C:
			processed = len(results)
			fmt.Printf("\rChecking records... %d/%d completed", processed, totalIPs)
		}
	}
}
