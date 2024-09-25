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

Usage:
  chk [options] <domain/subdomain/IP>

Options:
  -4: Show only IPv4 (A) records
  -6: Show only IPv6 (AAAA) records
  -h or --help: Display help information

GitHub Repository: https://github.com/kofany/chk

Author: Jerzy Dąbrowski

License: MIT License (https://kofany.mit-license.org)
*/
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

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
	IP      string
	PTR     []string
	IPInfo  *IPInfo
	IsIPv6  bool
	Error   error
}

var (
	cyan    = color.New(color.FgCyan).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
)

func getIPInfo(ip string) (*IPInfo, error) {
	resp, err := http.Get("https://ipinfo.io/" + ip + "/json")
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

func lookupIP(ip string, isIPv6 bool) Result {
	result := Result{IP: ip, IsIPv6: isIPv6}

	names, err := net.LookupAddr(ip)
	if err != nil {
		result.Error = fmt.Errorf("error looking up PTR records: %v", err)
	} else {
		result.PTR = names
	}

	ipInfo, err := getIPInfo(ip)
	if err != nil {
		if result.Error != nil {
			result.Error = fmt.Errorf("%v; error fetching IP info: %v", result.Error, err)
		} else {
			result.Error = fmt.Errorf("error fetching IP info: %v", err)
		}
	} else {
		result.IPInfo = ipInfo
	}

	return result
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

func showHelp() {
	fmt.Println(magenta("Extended DNS Check (chk) - Usage:"))
	fmt.Println(yellow("  chk <domain/subdomain>") + "              - Show A and AAAA records and IP info")
	fmt.Println(yellow("  chk -4 <domain/subdomain>") + "           - Show only A records and IP info")
	fmt.Println(yellow("  chk -6 <domain/subdomain>") + "           - Show only AAAA records and IP info")
	fmt.Println(yellow("  chk -h") + " or " + yellow("chk --help") + "              - Display this help")
}

func main() {
	if len(os.Args) < 2 {
		showHelp()
		os.Exit(1)
	}

	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		showHelp()
		os.Exit(0)
	}

	var input string
	var ipv4Only, ipv6Only bool

	switch os.Args[1] {
	case "-4":
		if len(os.Args) != 3 {
			fmt.Println(red("Error: Missing argument for -4 option"))
			os.Exit(1)
		}
		ipv4Only = true
		input = os.Args[2]
	case "-6":
		if len(os.Args) != 3 {
			fmt.Println(red("Error: Missing argument for -6 option"))
			os.Exit(1)
		}
		ipv6Only = true
		input = os.Args[2]
	default:
		input = os.Args[1]
	}

	ip := net.ParseIP(input)

	var results []Result
	var wg sync.WaitGroup

	if ip != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results = append(results, lookupIP(ip.String(), ip.To4() == nil))
		}()
	} else {
		ips, err := net.LookupIP(input)
		if err != nil {
			fmt.Printf("%s: %v\n", red("Error looking up IP for domain"), red(err))
			os.Exit(1)
		}

		for _, ip := range ips {
			isIPv6 := ip.To4() == nil
			if (ipv4Only && !isIPv6) || (ipv6Only && isIPv6) || (!ipv4Only && !ipv6Only) {
				wg.Add(1)
				go func(ip net.IP) {
					defer wg.Done()
					results = append(results, lookupIP(ip.String(), isIPv6))
				}(ip)
			}
		}
	}

	done := make(chan bool)
	go func() {
		wg.Wait()
		close(done)
	}()

	fmt.Print(yellow("Checking records... Please wait"))
	for {
		select {
		case <-done:
			fmt.Print("\r" + strings.Repeat(" ", 40) + "\r") // Clear the "Checking records..." message
			for _, result := range results {
				printResult(result)
			}
			return
		default:
			fmt.Print(".")
			time.Sleep(500 * time.Millisecond)
		}
	}
}
