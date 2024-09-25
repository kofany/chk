# Extended DNS Check (chk)

Extended DNS Check is a command-line tool that provides enhanced DNS lookup functionality. It checks PTR records for IP addresses and fetches additional IP information using the ipinfo.io API. For domains or subdomains, it displays A and AAAA records and retrieves IP information for each resolved address.

## Features

- Lookup A and AAAA records for domains and subdomains
- Retrieve PTR records for IP addresses
- Fetch detailed IP information (city, region, country, etc.) using ipinfo.io API
- Support for IPv4 and IPv6 addresses
- Colorized output for better readability

## Prerequisites

- Go 1.11 or higher
- Internet connection (for API calls)

## Installation

1. Clone the repository:
git clone https://github.com/kofany/chk.git
cd chk
 
2. Initialize the Go module:
go mod init github.com/kofany/chk
 
3. Install the required dependency:
go get github.com/fatih/color
 
## Building the Binary

To build the `chk` binary, run the following command in the project directory:
go build -o chk
 
This will create an executable named `chk` in your current directory.

## Usage
./chk [options] <domain/subdomain/IP>
 
Options:
- `-4`: Show only IPv4 (A) records
- `-6`: Show only IPv6 (AAAA) records
- `-h` or `--help`: Display help information

Examples:
./chk google.com
./chk -4 google.com
./chk -6 google.com
./chk 8.8.8.8
 
## Output

The tool provides colorized output with the following information:
- A/AAAA records (for domain lookups)
- PTR records
- IP information: City, Region, Country, Location, Organization

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](https://kofany.mit-license.org) file for details.

## Acknowledgments

- [ipinfo.io](https://ipinfo.io/) for providing IP information API
- [fatih/color](https://github.com/fatih/color) for colorized console output
