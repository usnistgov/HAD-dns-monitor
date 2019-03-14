# dns-monitor - A DNSSEC Monitor Written in Go

## Data Structure
The tool runs a series of DNS tests using a list of zones as input.  The results are stored in structures, which are then converted to BSON
objects and stored in a MongoDB database.  The database is expected to have a DB called "dns", in which the tool will store all the 
documents in a collection **dns.gov**

* Zone name (string)
* Agency name (org name) (string)
* Time checked (as 64bit int)
* DNS zone serial number (32 bit unsigned int)
* DNSSEC status (string)
* KSKs (array of 16 bit unsigned ints)
* ZSKs (array of 16 bit unsigned ints)
* Algorithms (array of 8 bit unsigned ints)
* NS set (array of strings)
* DS Hash algorithms used (array of 8 bit unsigned ints)


## To run the dns-monitor:

`dns-monitor -config=<configuration filename>`

where:

* config: the configuration file name.  See monitor.conf for an example.


## Input:
The input for the dns-monitor is a CSV file.  Each input line has the format:

Zone,Agency,Location

Only the first two values are used.  The original version of the file was gotten from [data.gov](https://home.dotgov.gov/data/)